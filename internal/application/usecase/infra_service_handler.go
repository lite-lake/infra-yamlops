package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

type InfraServiceHandler struct{}

func NewInfraServiceHandler() *InfraServiceHandler {
	return &InfraServiceHandler{}
}

func (h *InfraServiceHandler) EntityType() string {
	return "infra_service"
}

func (h *InfraServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	deployCtx, result := PrepareServiceDeploy(change, deps)
	if result != nil {
		return result, nil
	}

	if _, ok := deps.ServerInfo(deployCtx.ServerName); !ok {
		result = &Result{Change: change, Success: false}
		result.Error = fmt.Errorf("%w: %s", domainerr.ErrServerNotRegistered, deployCtx.ServerName)
		return result, nil
	}

	if change.Type() == valueobject.ChangeTypeDelete {
		return DeleteServiceRemote(change, deployCtx.Client, deployCtx.RemoteDir)
	}

	// 下发前按当前全量配置重渲染该服务产物（内部PRD）：磁盘 deployments/ 缓存
	// 可能被旧二进制 dry-run/异进程覆写（首例即 tracker_shipper 段被旧版静默丢弃），
	// 不作下发源。fail-closed——重渲染失败必须让该变更失败，绝不回落到陈旧缓存下发。
	if err := RefreshServiceArtifacts(deps, change.Name()); err != nil {
		result := &Result{Change: change, Success: false}
		result.Error = fmt.Errorf("refresh deployment artifacts for %s: %w", change.Name(), err)
		return result, nil
	}

	infra, _ := change.NewState().(*entity.InfraService)
	return ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{
		PreDeployHook:  h.createInfraTypePreHook(infra, change.Name(), deployCtx, deps),
		PostDeployHook: h.createInfraTypePostHook(infra, change.Name(), deployCtx, deps),
	})
}

func (h *InfraServiceHandler) SyncInfraFiles(serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) error {
	gatewayFile := h.getGatewayFilePath(deployCtx.ServerName, serviceName, deps)
	if gatewayFile != "" {
		if _, err := os.Stat(gatewayFile); err == nil {
			content, err := os.ReadFile(gatewayFile)
			if err != nil {
				return fmt.Errorf("%w: gateway file %s: %w", domainerr.ErrFileReadFailed, gatewayFile, err)
			}
			if err := SyncContent(deployCtx.Client, string(content), deployCtx.RemoteDir+"/gateway.yml"); err != nil {
				return fmt.Errorf("%w: gateway file %s to %s/gateway.yml: %w", domainerr.ErrComposeSyncFailed, gatewayFile, deployCtx.RemoteDir, err)
			}
		}
	}

	return nil
}

func (h *InfraServiceHandler) createInfraTypePreHook(infra *entity.InfraService, serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return func(result *Result) error {
		if infra == nil {
			return nil
		}
		switch infra.Type {
		case entity.InfraServiceTypeGateway:
			// cache 目录是 compose 生成器对网关的隐含挂载（./cache:/app/cache），
			// 不在网关服务的 volumes 配置里，需在容器启动前创建并赋予写权限，
			// 否则容器内 appuser(uid 1001) 无法写入，导致证书缓存保存失败。
			if err := h.prepareGatewayCacheDir(deployCtx); err != nil {
				result.Error = err
				return err
			}
			if err := h.deployGatewayType(serviceName, deployCtx, deps); err != nil {
				result.Error = err
				return err
			}
		case entity.InfraServiceTypeTunnelServer:
			if err := h.deployTunnelConfigFile(serviceName, deployCtx, deps, "frps.toml"); err != nil {
				result.Error = err
				return err
			}
		case entity.InfraServiceTypeTunnelClient:
			if err := h.deployTunnelConfigFile(serviceName, deployCtx, deps, "frpc.toml"); err != nil {
				result.Error = err
				return err
			}
		case entity.InfraServiceTypeEasyTier:
			if err := h.deployTunnelConfigFile(serviceName, deployCtx, deps, "easytier.toml"); err != nil {
				result.Error = err
				return err
			}
		case entity.InfraServiceTypeUptimeKuma:
			// 主容器隐含挂载 ./data:/app/data，需在启动前创建（Docker 首启自动创建为 root 属主，
			// 容器内非 root 用户无法写入），并同步 agent 消费的 kuma.targets.yaml。
			if err := h.prepareUptimeKumaDataDir(deployCtx); err != nil {
				result.Error = err
				return err
			}
			if err := h.deployTunnelConfigFile(serviceName, deployCtx, deps, "kuma.targets.yaml"); err != nil {
				result.Error = err
				return err
			}
		}
		return nil
	}
}

// createInfraTypePostHook 部署成功后的类型化后置钩子。
// 网关类型（内部PRD §6.4，D5 编译内嵌）：容器起来后校验远端渲染产物
// gateway.yml 的 tracker_shipper 不变式（enabled 与部署源实体一致）。校验只读
// （sudo cat），失败判部署失败（result.Error 置位 → RESULT 计入 failed、CLI 退出码非零），
// 不自动回滚。其余类型暂无后置动作。
func (h *InfraServiceHandler) createInfraTypePostHook(infra *entity.InfraService, serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return func(result *Result) error {
		if infra == nil || infra.Type != entity.InfraServiceTypeGateway {
			return nil
		}
		verifier := NewGatewayShipperVerifier(deps.Env())
		check := verifier.VerifyNode(deployCtx.Client, deployCtx.ServerName, infra)
		if check.Pass {
			return nil
		}
		result.Error = check.Err
		return check.Err
	}
}

func (h *InfraServiceHandler) createInfraTypeHook(infra *entity.InfraService, serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return h.createInfraTypePostHook(infra, serviceName, deployCtx, deps)
}

func (h *InfraServiceHandler) deployGatewayType(serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) error {
	gatewayFile := h.getGatewayFilePath(deployCtx.ServerName, serviceName, deps)
	if gatewayFile == "" {
		return nil
	}

	if _, err := os.Stat(gatewayFile); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(gatewayFile)
	if err != nil {
		return fmt.Errorf("%w: gateway file %s: %w", domainerr.ErrFileReadFailed, gatewayFile, err)
	}

	if err := SyncContent(deployCtx.Client, string(content), deployCtx.RemoteDir+"/gateway.yml"); err != nil {
		return fmt.Errorf("%w: gateway file %s to %s/gateway.yml: %w", domainerr.ErrComposeSyncFailed, gatewayFile, deployCtx.RemoteDir, err)
	}

	return nil
}

// prepareGatewayCacheDir 在容器启动前创建网关 cache 目录并赋予写权限。
// compose 生成器对网关类型隐含挂载 ./cache:/app/cache，该目录不在网关服务的
// volumes 配置里，Docker 首次启动自动创建时为 root:root(755)，容器内
// appuser(uid 1001) 无法写入，导致证书缓存保存失败。此处显式 chmod 777。
func (h *InfraServiceHandler) prepareGatewayCacheDir(deployCtx *ServiceDeployContext) error {
	cacheDir := deployCtx.RemoteDir + "/cache"
	cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chmod -R 777 %s",
		ssh.ShellEscape(cacheDir),
		ssh.ShellEscape(cacheDir))
	if _, stderr, err := deployCtx.Client.Run(cmd); err != nil {
		return fmt.Errorf("%w: prepare gateway cache dir %s: %s", domainerr.ErrDirectoryCreateFailed, cacheDir, stderr)
	}
	return nil
}

// prepareUptimeKumaDataDir 在容器启动前创建 uptime-kuma 的数据目录 ./data 并赋予写权限
// （compose 隐含挂载 ./data:/app/data，Docker 首启创建为 root:root，容器内非 root 用户无法写入）。
// 仅对目录本身 chmod 777，不做 -R：data 内包含 db-config.json（明文 DB 密码）等敏感文件，
// 递归放开会让任意用户可读。
func (h *InfraServiceHandler) prepareUptimeKumaDataDir(deployCtx *ServiceDeployContext) error {
	dataDir := deployCtx.RemoteDir + "/data"
	cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chmod 777 %s",
		ssh.ShellEscape(dataDir),
		ssh.ShellEscape(dataDir))
	if _, stderr, err := deployCtx.Client.Run(cmd); err != nil {
		return fmt.Errorf("%w: prepare uptime-kuma data dir %s: %s", domainerr.ErrDirectoryCreateFailed, dataDir, stderr)
	}
	return nil
}

func (h *InfraServiceHandler) getGatewayFilePath(serverName, serviceName string, deps DepsProvider) string {
	return filepath.Join(deps.WorkDir(), "deployments", deps.Env(), serverName, serviceName+".gate.yaml")
}

func (h *InfraServiceHandler) deployTunnelConfigFile(serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider, configSuffix string) error {
	localFile := filepath.Join(deps.WorkDir(), "deployments", deps.Env(), deployCtx.ServerName, serviceName+"."+configSuffix)
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		return nil
	}
	content, err := os.ReadFile(localFile)
	if err != nil {
		return fmt.Errorf("%w: tunnel config %s: %w", domainerr.ErrFileReadFailed, localFile, err)
	}
	remotePath := deployCtx.RemoteDir + "/" + serviceName + "." + configSuffix
	if err := SyncContent(deployCtx.Client, string(content), remotePath); err != nil {
		return fmt.Errorf("%w: tunnel config %s to %s: %w", domainerr.ErrComposeSyncFailed, localFile, remotePath, err)
	}
	return nil
}
