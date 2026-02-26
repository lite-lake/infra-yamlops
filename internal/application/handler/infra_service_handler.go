package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
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

	infra, _ := change.NewState().(*entity.InfraService)
	return ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{
		PostDeployHook: h.createInfraTypeHook(infra, change.Name(), deployCtx, deps),
		RestartAfterUp: true,
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

	sslConfigFile := h.getSSLConfigFilePathFromWorkDir(deps)
	if sslConfigFile != "" {
		if _, err := os.Stat(sslConfigFile); err == nil {
			content, err := os.ReadFile(sslConfigFile)
			if err != nil {
				return fmt.Errorf("%w: SSL config file %s: %w", domainerr.ErrFileReadFailed, sslConfigFile, err)
			}
			if err := deployCtx.Client.MkdirAllSudoWithPerm(deployCtx.RemoteDir+"/ssl-config", "755"); err != nil {
				return fmt.Errorf("%w: ssl-config directory at %s/ssl-config: %w", domainerr.ErrDirectoryCreateFailed, deployCtx.RemoteDir, err)
			}
			if err := SyncContent(deployCtx.Client, string(content), deployCtx.RemoteDir+"/ssl-config/config.yml"); err != nil {
				return fmt.Errorf("%w: SSL config file %s to %s/ssl-config/config.yml: %w", domainerr.ErrComposeSyncFailed, sslConfigFile, deployCtx.RemoteDir, err)
			}
		}
	}

	return nil
}

func (h *InfraServiceHandler) getSSLConfigFilePathFromWorkDir(deps DepsProvider) string {
	return filepath.Join(deps.WorkDir(), "userdata", deps.Env(), "volumes", "ssl", "config.yml")
}

func (h *InfraServiceHandler) createInfraTypeHook(infra *entity.InfraService, serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return func(result *Result) error {
		if infra != nil && infra.Type == entity.InfraServiceTypeGateway {
			if err := h.deployGatewayType(serviceName, deployCtx, deps); err != nil {
				result.Error = err
				return err
			}
		}

		if infra != nil && infra.Type == entity.InfraServiceTypeSSL {
			if err := h.deploySSLType(infra, deployCtx, deps); err != nil {
				result.Error = err
				return err
			}
		}
		return nil
	}
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

func (h *InfraServiceHandler) deploySSLType(infra *entity.InfraService, deployCtx *ServiceDeployContext, deps DepsProvider) error {
	if infra == nil || infra.SSLConfig == nil || infra.SSLConfig.Config == nil {
		return nil
	}

	sslConfigFile := h.getSSLConfigFilePath(infra, deps)
	if sslConfigFile == "" {
		return nil
	}

	if _, err := os.Stat(sslConfigFile); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", domainerr.ErrFileNotFound, sslConfigFile)
	}

	content, err := os.ReadFile(sslConfigFile)
	if err != nil {
		return fmt.Errorf("%w: SSL config file %s: %w", domainerr.ErrFileReadFailed, sslConfigFile, err)
	}

	if err := deployCtx.Client.MkdirAllSudoWithPerm(deployCtx.RemoteDir+"/ssl-config", "755"); err != nil {
		return fmt.Errorf("%w: ssl-config directory at %s/ssl-config: %w", domainerr.ErrDirectoryCreateFailed, deployCtx.RemoteDir, err)
	}

	if err := SyncContent(deployCtx.Client, string(content), deployCtx.RemoteDir+"/ssl-config/config.yml"); err != nil {
		return fmt.Errorf("%w: SSL config file %s to %s/ssl-config/config.yml: %w", domainerr.ErrComposeSyncFailed, sslConfigFile, deployCtx.RemoteDir, err)
	}

	return nil
}

func (h *InfraServiceHandler) getGatewayFilePath(serverName, serviceName string, deps DepsProvider) string {
	return filepath.Join(deps.WorkDir(), "deployments", serverName, serviceName+".gate.yaml")
}

func (h *InfraServiceHandler) getSSLConfigFilePath(infra *entity.InfraService, deps DepsProvider) string {
	if infra.SSLConfig == nil || infra.SSLConfig.Config == nil || infra.SSLConfig.Config.Source == "" {
		return ""
	}

	source := infra.SSLConfig.Config.Source
	if !strings.HasPrefix(source, "volumes://") {
		return ""
	}

	volumePath := strings.TrimPrefix(source, "volumes://")
	return filepath.Join(deps.WorkDir(), "userdata", deps.Env(), "volumes", volumePath, "config.yml")
}
