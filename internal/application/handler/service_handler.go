package handler

import (
	"context"
	"fmt"
	"strings"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
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
		PreDeployHook:  h.createPreDeployHook(change, deployCtx, deps),
		PostDeployHook: nil,
		RestartAfterUp: false,
	})
}

func (h *ServiceHandler) createPreDeployHook(change *valueobject.Change, deployCtx *ServiceDeployContext, deps DepsProvider) func(*Result) error {
	return func(result *Result) error {
		if change.NewState() == nil {
			return nil
		}

		svc, ok := change.NewState().(*entity.BizService)
		if !ok {
			return fmt.Errorf("invalid state type: %T", change.NewState())
		}

		// 1. Handle registry login first
		if svc.Registry != "" {
			registryMgr, err := deps.RegistryManager(deployCtx.ServerName)
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
		}

		// 2. Create volume directories with 777 permissions
		for _, vol := range svc.Volumes {
			// Extract source path from volume (remove ./ prefix if present)
			volumeSource := vol.Source
			if strings.HasPrefix(volumeSource, "./") {
				volumeSource = volumeSource[2:]
			}
			if strings.HasPrefix(volumeSource, "/") {
				volumeSource = volumeSource[1:]
			}
			volumePath := deployCtx.RemoteDir + "/" + volumeSource

			// Create directory with 777 permissions recursively
			cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chmod -R 777 %s",
				ssh.ShellEscape(volumePath),
				ssh.ShellEscape(volumePath))

			_, stderr, err := deployCtx.Client.Run(cmd)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to prepare volume %s: %s", vol.Source, stderr))
			}
		}

		return nil
	}
}
