package handler

import (
	"context"
	"fmt"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type ServiceHandler struct{}

func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

func (h *ServiceHandler) EntityType() string {
	return "service"
}

func (h *ServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	deployCtx, result := PrepareServiceDeploy(change, deps)
	if result != nil {
		return result, nil
	}

	if change.Type() == valueobject.ChangeTypeDelete {
		return DeleteServiceRemote(change, deployCtx.Client, deployCtx.RemoteDir)
	}

	return ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{
		PreDeployHook:  h.createRegistryLoginHook(change, deps, deployCtx.ServerName),
		RestartAfterUp: false,
	})
}

func (h *ServiceHandler) createRegistryLoginHook(change *valueobject.Change, deps DepsProvider, serverName string) func(*Result) error {
	return func(result *Result) error {
		if change.NewState() == nil {
			return nil
		}

		svc, ok := change.NewState().(*entity.BizService)
		if !ok {
			return fmt.Errorf("invalid state type: %T", change.NewState())
		}

		if svc.Registry == "" {
			return nil
		}

		registryMgr, err := deps.RegistryManager(serverName)
		if err != nil {
			result.Error = fmt.Errorf("get registry manager: %w", err)
			return err
		}

		loginResult, err := registryMgr.EnsureLoggedIn(svc.Registry)
		if err != nil {
			result.Error = fmt.Errorf("login registry %s: %w", svc.Registry, err)
			return err
		}

		if !loginResult.Success {
			result.Error = fmt.Errorf("%w: %s", domainerr.ErrRegistryLoginFailed, loginResult.Message)
			return fmt.Errorf("%w: %s", domainerr.ErrRegistryLoginFailed, loginResult.Message)
		}

		return nil
	}
}
