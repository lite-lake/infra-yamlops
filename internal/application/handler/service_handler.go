package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
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
		RestartAfterUp: true,
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

		// 2. Create volume directories and sync content if needed
		for _, vol := range svc.Volumes {
			volumeSource := vol.Source

			// Handle volumes:// protocol
			isRemoteVolume := strings.HasPrefix(volumeSource, "volumes://")
			var localVolumePath string
			var volumeName string
			if isRemoteVolume {
				volumeName = strings.TrimPrefix(volumeSource, "volumes://")
				localVolumePath = filepath.Join(deps.WorkDir(), "userdata", deps.Env(), "volumes", volumeName)
			} else {
				// Extract source path from volume (remove ./ prefix if present)
				if strings.HasPrefix(volumeSource, "./") {
					volumeSource = volumeSource[2:]
				}
				if strings.HasPrefix(volumeSource, "/") {
					volumeSource = volumeSource[1:]
				}
			}

			// Create directory with 777 permissions recursively
			var targetDir string
			if isRemoteVolume {
				targetDir = deployCtx.RemoteDir + "/" + volumeName
			} else {
				targetDir = deployCtx.RemoteDir + "/" + volumeSource
			}

			cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chmod -R 777 %s",
				ssh.ShellEscape(targetDir),
				ssh.ShellEscape(targetDir))

			_, stderr, err := deployCtx.Client.Run(cmd)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to prepare volume %s: %s", vol.Source, stderr))
			}

			// Sync content if sync is enabled and it's a remote volume
			if isRemoteVolume && vol.Sync {
				if err := h.syncVolumeContent(deployCtx.Client, localVolumePath, targetDir); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to sync volume %s: %s", vol.Source, err.Error()))
				}
			}
		}

		return nil
	}
}

func (h *ServiceHandler) syncVolumeContent(client contract.SSHClient, localDir, remoteDir string) error {
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		return fmt.Errorf("volume directory not found: %s", localDir)
	}

	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		// Use forward slashes for remote Linux paths
		remotePath := remoteDir + "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chmod -R 777 %s",
				ssh.ShellEscape(remotePath),
				ssh.ShellEscape(remotePath))
			_, stderr, err := client.Run(cmd)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %s", remotePath, stderr)
			}
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			if err := SyncContent(client, string(content), remotePath); err != nil {
				return fmt.Errorf("failed to sync file %s: %w", path, err)
			}
		}

		return nil
	})
}
