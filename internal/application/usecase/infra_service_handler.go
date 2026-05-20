package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
		PreDeployHook:  h.createInfraTypePreHook(infra, change.Name(), deployCtx, deps),
		PostDeployHook: h.createInfraTypePostHook(infra, change.Name(), deployCtx, deps),
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

	return nil
}

func (h *InfraServiceHandler) createInfraTypePreHook(infra *entity.InfraService, serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return func(result *Result) error {
		if infra != nil && infra.Type == entity.InfraServiceTypeGateway {
			if err := h.deployGatewayType(serviceName, deployCtx, deps); err != nil {
				result.Error = err
				return err
			}
		}
		return nil
	}
}

func (h *InfraServiceHandler) createInfraTypePostHook(infra *entity.InfraService, serviceName string, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return func(result *Result) error {
		return nil
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

func (h *InfraServiceHandler) getGatewayFilePath(serverName, serviceName string, deps DepsProvider) string {
	return filepath.Join(deps.WorkDir(), "deployments", deps.Env(), serverName, serviceName+".gate.yaml")
}
