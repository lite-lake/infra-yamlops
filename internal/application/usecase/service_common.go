package usecase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/network"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

type NetworkGetter interface {
	GetNetworks() []string
}

func GetRequiredNetworks(change *valueobject.Change, deps DepsProvider, serverName string) ([]entity.ServerNetwork, error) {
	server, ok := deps.Server(serverName)
	if !ok {
		return nil, fmt.Errorf("%w: %s", domainerr.ErrServerNotRegistered, serverName)
	}

	var serviceNetworks []string
	if change.NewState() != nil {
		if getter, ok := change.NewState().(NetworkGetter); ok {
			serviceNetworks = getter.GetNetworks()
		} else if svc, ok := change.NewState().(map[string]interface{}); ok {
			if nets, ok := svc["networks"].([]interface{}); ok {
				for _, n := range nets {
					if name, ok := n.(string); ok {
						serviceNetworks = append(serviceNetworks, name)
					}
				}
			}
		}
	}

	if len(serviceNetworks) == 0 {
		if len(server.Networks) > 0 {
			return server.Networks[:1], nil
		}
		return []entity.ServerNetwork{{Name: fmt.Sprintf("yamlops-%s", deps.Env()), Type: entity.NetworkTypeBridge}}, nil
	}

	var requiredNetworks []entity.ServerNetwork
	for _, netName := range serviceNetworks {
		if net, ok := server.GetNetwork(netName); ok {
			requiredNetworks = append(requiredNetworks, net)
		} else {
			requiredNetworks = append(requiredNetworks, entity.ServerNetwork{Name: netName, Type: entity.NetworkTypeBridge})
		}
	}
	return requiredNetworks, nil
}

func EnsureNetworks(client contract.SSHRunner, networks []entity.ServerNetwork) error {
	if len(networks) == 0 {
		return nil
	}
	netMgr := network.NewManager(client)
	for _, netSpec := range networks {
		if err := netMgr.Ensure(&netSpec); err != nil {
			return fmt.Errorf("ensure network %s: %w", netSpec.Name, err)
		}
	}
	return nil
}

func DeleteServiceRemote(change *valueobject.Change, client contract.SSHRunner, remoteDir string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	escapedDir := ssh.ShellEscape(remoteDir)
	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml down -v 2>/dev/null || true", escapedDir)
	stdout, stderr, _ := client.Run(cmd)

	rmCmd := fmt.Sprintf("sudo rm -rf %s", escapedDir)
	stdout2, stderr2, err := client.Run(rmCmd)
	if err != nil {
		result.Error = fmt.Errorf("%w: %w, stderr: %s", domainerr.ErrDirectoryRemoveFailed, err, stderr2)
		result.Output = stdout + "\n" + stderr + "\n" + stdout2 + "\n" + stderr2
		return result, nil
	}

	result.Success = true
	result.Output = stdout + "\n" + stdout2
	return result, nil
}

func GetComposeFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", deps.Env(), serverName, ch.Name()+".compose.yaml")
}

// prodPrevTag 是回滚目标镜像的固定标签。每次部署前把即将被替换的镜像打上该标签，
// 保证任何时刻都有上一版可用镜像；prune 不再删除带标签镜像（见 environment.prune）。
const prodPrevTag = "prod-prev"

type DeployComposeConfig struct {
	RemoteDir   string
	ComposeFile string
	EnvFile     string
	// EnvFilePerm 远端 env 文件的权限（chmod 八进制字符串，如 "600"）。空则默认 644。
	EnvFilePerm string
	// ContainerName 部署前要保留旧镜像的容器名。空则不保留（如未容器化的场景）。
	ContainerName string
	// Env 部署环境。只有 prod 才保留 :prod-prev 回滚镜像，避免 dev 镜像被误标。
	Env string
	// Image 本次部署期望的镜像引用（来自实体 image 字段）。用于识别回滚部署并跳过
	// 对其自身的保留/拉取。
	Image string
}

// RetainPreviousImage 在拉取新镜像前，把容器当前运行的镜像打上 <repo>:prod-prev，
// 作为回滚目标。尽力而为：首次部署无旧镜像时静默返回，打标签失败仅记 Warning，
// 不阻塞部署。
//
// 回滚部署（desiredImage 或容器当前镜像已是 :prod-prev）不做任何改动：否则会把
// 回滚目标覆盖成自身，导致下一次回滚无镜像可用。
func RetainPreviousImage(client contract.SSHRunner, containerName, desiredImage string, result *Result) {
	// 期望镜像本身就是 :prod-prev → 这是回滚/演练恢复，不能覆盖回滚目标。
	if isProdPrevRef(desiredImage) {
		return
	}

	inspectCmd := fmt.Sprintf("sudo docker inspect -f '{{.Image}}|{{.Config.Image}}' %s 2>/dev/null", ssh.ShellEscape(containerName))
	stdout, _, err := client.Run(inspectCmd)
	// inspect 失败（容器不存在等）→ 首次部署，没有旧镜像可保留。
	if err != nil {
		return
	}

	parts := strings.SplitN(strings.TrimSpace(stdout), "|", 2)
	if len(parts) != 2 {
		return
	}
	imageID := strings.TrimSpace(parts[0])
	currentRef := strings.TrimSpace(parts[1])
	if imageID == "" || currentRef == "" || currentRef == "<no value>" {
		return
	}

	// 当前运行的就是回滚目标镜像（digest 感知），不能用它自己覆盖回滚目标。
	if isProdPrevRef(currentRef) {
		return
	}

	repo := stripImageTag(currentRef)
	if repo == "" {
		return
	}

	tagCmd := fmt.Sprintf("sudo docker tag %s %s", ssh.ShellEscape(imageID), ssh.ShellEscape(repo+":"+prodPrevTag))
	if _, stderr, tagErr := client.Run(tagCmd); tagErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"retain previous image %s as %s:%s: %v, stderr: %s",
			imageID, repo, prodPrevTag, tagErr, strings.TrimSpace(stderr)))
	}
}

// stripImageDigest 去掉 @sha256:... 等 digest 后缀（保留 tag）。
func stripImageDigest(ref string) string {
	ref = strings.TrimSpace(ref)
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	return ref
}

// isProdPrevRef 判断镜像引用（可带 digest）是否指向 :prod-prev。
func isProdPrevRef(ref string) bool {
	return strings.HasSuffix(stripImageDigest(ref), ":"+prodPrevTag)
}

// stripImageTag 去掉镜像引用上的 tag 与 @sha256 digest，得到仓库名。
// 兼容带端口 registry：host:5000/repo:tag → host:5000/repo；
// host:5000/repo（无 tag）保持原样；repo@sha256:... → repo。
func stripImageTag(ref string) string {
	ref = stripImageDigest(ref)
	lastSlash := strings.LastIndex(ref, "/")
	if i := strings.LastIndex(ref, ":"); i > lastSlash {
		ref = ref[:i]
	}
	return ref
}

func DeployComposeFile(client contract.SSHClient, cfg *DeployComposeConfig, result *Result) bool {
	if cfg.ComposeFile == "" {
		return true
	}

	if _, err := os.Stat(cfg.ComposeFile); err != nil {
		return true
	}

	content, err := os.ReadFile(cfg.ComposeFile)
	if err != nil {
		result.Error = fmt.Errorf("%w: compose file %s: %w", domainerr.ErrFileReadFailed, cfg.ComposeFile, err)
		return false
	}
	if err := SyncContent(client, string(content), cfg.RemoteDir+"/docker-compose.yml"); err != nil {
		result.Error = fmt.Errorf("%w: compose file %s to %s/docker-compose.yml: %w", domainerr.ErrComposeSyncFailed, cfg.ComposeFile, cfg.RemoteDir, err)
		return false
	}

	if cfg.EnvFile != "" {
		if _, err := os.Stat(cfg.EnvFile); err == nil {
			envContent, err := os.ReadFile(cfg.EnvFile)
			if err != nil {
				result.Error = fmt.Errorf("%w: env file %s: %w", domainerr.ErrFileReadFailed, cfg.EnvFile, err)
				return false
			}
			envFileName := filepath.Base(cfg.EnvFile)
			if cfg.EnvFilePerm != "" {
				if err := SyncContentWithPerm(client, string(envContent), cfg.RemoteDir+"/"+envFileName, cfg.EnvFilePerm); err != nil {
					result.Error = fmt.Errorf("%w: env file %s to %s/%s: %v", domainerr.ErrComposeSyncFailed, cfg.EnvFile, cfg.RemoteDir, envFileName, err)
					return false
				}
			} else if err := SyncContent(client, string(envContent), cfg.RemoteDir+"/"+envFileName); err != nil {
				result.Error = fmt.Errorf("%w: env file %s to %s/%s: %v", domainerr.ErrComposeSyncFailed, cfg.EnvFile, cfg.RemoteDir, envFileName, err)
				return false
			}
		}
	}

	// 只有 prod 才保留上一版镜像为 :prod-prev，dev/test 镜像不参与回滚。
	if cfg.Env == "prod" && cfg.ContainerName != "" {
		RetainPreviousImage(client, cfg.ContainerName, cfg.Image, result)
	}

	// 回滚部署：期望镜像 :prod-prev 若已在本地，直接以本地保留镜像为准，跳过 pull
	// （它可能已不在 registry，或 registry 上的同名 tag 已漂移）。
	skipPull := false
	if isProdPrevRef(cfg.Image) {
		inspectImageCmd := fmt.Sprintf("sudo docker image inspect %s >/dev/null 2>&1", ssh.ShellEscape(strings.TrimSpace(cfg.Image)))
		if _, _, inspectErr := client.Run(inspectImageCmd); inspectErr == nil {
			skipPull = true
		}
	}

	if !skipPull {
		pullCmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml pull", cfg.RemoteDir)
		_, pullStderr, pullErr := client.Run(pullCmd)
		if pullErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("image pull failed: %s", pullStderr))
		}
	}

	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d --force-recreate --pull never", cfg.RemoteDir)
	stdout, stderr, err := client.Run(cmd)
	if err != nil {
		result.Error = fmt.Errorf("%w: in %s: %w, stderr: %s", domainerr.ErrDockerComposeFailed, cfg.RemoteDir, err, stderr)
		result.Output = stdout + "\n" + stderr
		return false
	}
	result.Output = stdout
	return true
}

type RestartServiceConfig struct {
	RemoteDir   string
	ComposeFile string
	EnvFile     string
	Env         string
	ServiceName string
}

func RestartServiceWithFileSync(client contract.SSHClient, cfg *RestartServiceConfig, result *Result) bool {
	// restart 不销毁容器（日志保留），但归档一份可留下清晰的时间分界线，
	// 便于按运行批次查看日志。失败仅告警不阻塞重启。
	if archiveID, err := ArchiveContainerLogs(client, cfg.RemoteDir, ContainerName(cfg.Env, cfg.ServiceName), time.Now()); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("archive logs before restart: %v", err))
	} else if archiveID != "" {
		_ = PruneArchiveRuns(client, cfg.RemoteDir, constants.MaxLogsArchiveCount)
	}

	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml restart 2>&1", cfg.RemoteDir)
	stdout, stderr, err := client.Run(cmd)
	if err != nil {
		result.Error = fmt.Errorf("restart failed in %s: %w, stderr: %s", cfg.RemoteDir, err, stderr)
		result.Output = stdout + "\n" + stderr
		return false
	}
	result.Output = stdout
	return true
}

func GetRemoteDir(deps DepsProvider, serviceName string) string {
	return fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, deps.Env(), serviceName))
}

func EnsureRemoteDir(client contract.SSHClient, remoteDir string) error {
	return client.MkdirAllSudoWithPerm(remoteDir, "755")
}

type ServiceDeployContext struct {
	ServerName string
	Client     contract.SSHClient
	RemoteDir  string
}

type DeployServiceOptions struct {
	PreDeployHook  func(result *Result) error
	PostDeployHook func(result *Result) error
}

type ServiceRestartManager struct {
	deps DepsProvider
}

func NewServiceRestartManager(deps DepsProvider) *ServiceRestartManager {
	return &ServiceRestartManager{deps: deps}
}

func (m *ServiceRestartManager) RestartBizService(serverName, serviceName string) *Result {
	result := &Result{Success: false}

	client, err := m.deps.SSHClient(serverName)
	if err != nil {
		result.Error = err
		return result
	}

	// restart 同样消费磁盘 deployments/ 缓存（compose 路径取自缓存），下发（同步/读取）
	// 前先按当前全量配置重渲染刷新缓存（内部PRD）。fail-closed：失败即中止，
	// 绝不回落到陈旧缓存。
	if err := RefreshServiceArtifacts(m.deps, serviceName); err != nil {
		result.Error = fmt.Errorf("refresh deployment artifacts for %s: %w", serviceName, err)
		return result
	}

	remoteDir := GetRemoteDir(m.deps, serviceName)
	composeFile := filepath.Join(m.deps.WorkDir(), "deployments", m.deps.Env(), serverName, serviceName+".compose.yaml")
	envFile := composeFile[:len(composeFile)-len(".compose.yaml")] + ".env"

	if !RestartServiceWithFileSync(client, &RestartServiceConfig{
		RemoteDir:   remoteDir,
		ComposeFile: composeFile,
		EnvFile:     envFile,
		Env:         m.deps.Env(),
		ServiceName: serviceName,
	}, result) {
		return result
	}

	result.Success = true
	return result
}

func (m *ServiceRestartManager) RestartInfraService(serverName, serviceName string) *Result {
	result := &Result{Success: false}

	client, err := m.deps.SSHClient(serverName)
	if err != nil {
		result.Error = err
		return result
	}

	// restart 会经 SyncInfraFiles 把磁盘缓存的 gate 配置重新下发到远端，同属陈旧缓存
	// 下发缺陷面（内部PRD）：先按当前全量配置重渲染刷新缓存再同步。
	// fail-closed：失败即中止，绝不回落到陈旧缓存下发。
	if err := RefreshServiceArtifacts(m.deps, serviceName); err != nil {
		result.Error = fmt.Errorf("refresh deployment artifacts for %s: %w", serviceName, err)
		return result
	}

	remoteDir := GetRemoteDir(m.deps, serviceName)
	composeFile := filepath.Join(m.deps.WorkDir(), "deployments", m.deps.Env(), serverName, serviceName+".compose.yaml")

	deployCtx := &ServiceDeployContext{
		ServerName: serverName,
		Client:     client,
		RemoteDir:  remoteDir,
	}

	infraHandler := NewInfraServiceHandler()
	if err := infraHandler.SyncInfraFiles(serviceName, deployCtx, m.deps); err != nil {
		result.Error = err
		return result
	}

	if !RestartServiceWithFileSync(client, &RestartServiceConfig{
		RemoteDir:   remoteDir,
		ComposeFile: composeFile,
		EnvFile:     "",
		Env:         m.deps.Env(),
		ServiceName: serviceName,
	}, result) {
		return result
	}

	result.Success = true
	return result
}

func PrepareServiceDeploy(change *valueobject.Change, deps DepsProvider) (*ServiceDeployContext, *Result) {
	result := &Result{Change: change, Success: false}

	serverName := ExtractServerFromChange(change)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", change.Name())
		return nil, result
	}

	client, err := deps.SSHClient(serverName)
	if err != nil {
		result.Error = err
		return nil, result
	}

	remoteDir := GetRemoteDir(deps, change.Name())

	return &ServiceDeployContext{
		ServerName: serverName,
		Client:     client,
		RemoteDir:  remoteDir,
	}, nil
}

// desiredServiceImage 返回变更目标实体的镜像引用（业务/基础设施服务均有 image 字段）。
// 非服务实体或缺失时返回空串，调用方据此跳过镜像保留逻辑。
func desiredServiceImage(change *valueobject.Change) string {
	switch svc := change.NewState().(type) {
	case *entity.BizService:
		return strings.TrimSpace(svc.Image)
	case *entity.InfraService:
		return strings.TrimSpace(svc.Image)
	default:
		return ""
	}
}

func ExecuteServiceDeploy(change *valueobject.Change, ctx *ServiceDeployContext, deps DepsProvider, opts DeployServiceOptions) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if opts.PreDeployHook != nil {
		if err := opts.PreDeployHook(result); err != nil {
			return result, nil
		}
	}

	// deploy 用 --force-recreate 重建容器，旧容器日志随之销毁；
	// 重建前归档一份，供 service logs --previous/--run 查询。失败仅告警不阻塞部署。
	if archiveID, err := ArchiveContainerLogs(ctx.Client, ctx.RemoteDir, ContainerName(deps.Env(), change.Name()), time.Now()); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("archive logs before recreate: %v", err))
	} else if archiveID != "" {
		_ = PruneArchiveRuns(ctx.Client, ctx.RemoteDir, constants.MaxLogsArchiveCount)
	}

	requiredNetworks, err := GetRequiredNetworks(change, deps, ctx.ServerName)
	if err != nil {
		result.Error = err
		return result, nil
	}

	if err := EnsureNetworks(ctx.Client, requiredNetworks); err != nil {
		result.Error = fmt.Errorf("ensuring networks on server %s: %w", ctx.ServerName, err)
		return result, nil
	}

	if err := EnsureRemoteDir(ctx.Client, ctx.RemoteDir); err != nil {
		result.Error = fmt.Errorf("%w: %s: %w", domainerr.ErrDirectoryCreateFailed, ctx.RemoteDir, err)
		return result, nil
	}

	composeFile := GetComposeFilePath(change, deps)
	envFile := ""
	if composeFile != "" {
		envFile = composeFile[:len(composeFile)-len(".compose.yaml")] + ".env"
	}
	deployCfg := &DeployComposeConfig{
		RemoteDir:     ctx.RemoteDir,
		ComposeFile:   composeFile,
		EnvFile:       envFile,
		ContainerName: ContainerName(deps.Env(), change.Name()),
		Env:           deps.Env(),
		Image:         desiredServiceImage(change),
	}
	if !DeployComposeFile(ctx.Client, deployCfg, result) {
		return result, nil
	}

	if opts.PostDeployHook != nil {
		if err := opts.PostDeployHook(result); err != nil {
			return result, nil
		}
	}

	result.Success = true
	return result, nil
}
