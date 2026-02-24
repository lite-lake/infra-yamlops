package handler

import (
	"context"
	"fmt"

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
	result := &Result{Change: change, Success: false}

	serverName := ExtractServerFromChange(change)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", change.Name)
		return result, nil
	}

	client, err := deps.SSHClient(serverName)
	if err != nil {
		result.Error = err
		return result, nil
	}

	remoteDir := GetRemoteDir(deps, change.Name)

	switch change.Type {
	case valueobject.ChangeTypeDelete:
		return DeleteServiceRemote(change, client, remoteDir)
	default:
		return h.deployService(change, client, remoteDir, deps, serverName)
	}
}

func (h *ServiceHandler) deployService(change *valueobject.Change, client SSHClient, remoteDir string, deps DepsProvider, serverName string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if err := h.handleRegistryLogin(change, deps, serverName, result); err != nil {
		return result, nil
	}

	requiredNetworks, err := GetRequiredNetworks(change, deps, serverName)
	if err != nil {
		result.Error = err
		return result, nil
	}

	if err := EnsureNetworks(client, requiredNetworks); err != nil {
		result.Error = err
		return result, nil
	}

	if err := EnsureRemoteDir(client, remoteDir); err != nil {
		result.Error = fmt.Errorf("failed to create remote directory: %w", err)
		return result, nil
	}

	composeFile := GetComposeFilePath(change, deps)
	if !DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:      remoteDir,
		ComposeFile:    composeFile,
		Env:            deps.Env(),
		ServiceName:    change.Name,
		RestartAfterUp: false,
	}, result) {
		return result, nil
	}

	result.Success = true
	return result, nil
}

func (h *ServiceHandler) handleRegistryLogin(change *valueobject.Change, deps DepsProvider, serverName string, result *Result) error {
	if change.NewState == nil {
		return nil
	}

	svc, ok := change.NewState.(map[string]interface{})
	if !ok {
		return nil
	}

	registryName, exists := svc["registry"].(string)
	if !exists || registryName == "" {
		return nil
	}

	registryMgr, err := deps.RegistryManager(serverName)
	if err != nil {
		result.Error = fmt.Errorf("failed to get registry manager: %w", err)
		return err
	}

	loginResult, err := registryMgr.EnsureLoggedIn(registryName)
	if err != nil {
		result.Error = fmt.Errorf("failed to login registry %s: %w", registryName, err)
		return err
	}

	if !loginResult.Success {
		result.Error = fmt.Errorf("registry login failed: %s", loginResult.Message)
		return fmt.Errorf("registry login failed")
	}

	return nil
}
