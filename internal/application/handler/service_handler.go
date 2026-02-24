package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/network"
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

	remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, deps.Env(), change.Name))

	switch change.Type {
	case valueobject.ChangeTypeDelete:
		return h.deleteService(change, client, remoteDir)
	default:
		return h.deployService(change, client, remoteDir, deps, serverName)
	}
}

func (h *ServiceHandler) deployService(change *valueobject.Change, client SSHClient, remoteDir string, deps DepsProvider, serverName string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if change.NewState != nil {
		if svc, ok := change.NewState.(map[string]interface{}); ok {
			if registryName, exists := svc["registry"].(string); exists && registryName != "" {
				registryMgr, err := deps.RegistryManager(serverName)
				if err != nil {
					result.Error = fmt.Errorf("failed to get registry manager: %w", err)
					return result, nil
				}
				loginResult, err := registryMgr.EnsureLoggedIn(registryName)
				if err != nil {
					result.Error = fmt.Errorf("failed to login registry %s: %w", registryName, err)
					return result, nil
				}
				if !loginResult.Success {
					result.Error = fmt.Errorf("registry login failed: %s", loginResult.Message)
					return result, nil
				}
			}
		}
	}

	requiredNetworks, err := h.getRequiredNetworks(change, deps, serverName)
	if err != nil {
		result.Error = err
		return result, nil
	}

	if len(requiredNetworks) > 0 {
		netMgr := network.NewManager(client)
		for _, netSpec := range requiredNetworks {
			if err := netMgr.Ensure(&netSpec); err != nil {
				result.Error = fmt.Errorf("failed to ensure network %s: %w", netSpec.Name, err)
				return result, nil
			}
		}
	}

	if err := client.MkdirAllSudoWithPerm(remoteDir, "755"); err != nil {
		result.Error = fmt.Errorf("failed to create remote directory: %w", err)
		return result, nil
	}

	composeFile := h.getComposeFilePath(change, deps)
	if composeFile != "" {
		if _, err := os.Stat(composeFile); err == nil {
			content, err := os.ReadFile(composeFile)
			if err != nil {
				result.Error = fmt.Errorf("failed to read compose file: %w", err)
				return result, nil
			}
			if err := SyncContent(client, string(content), remoteDir+"/docker-compose.yml"); err != nil {
				result.Error = fmt.Errorf("failed to sync compose file: %w", err)
				return result, nil
			}

			pullCmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml pull", remoteDir)
			_, pullStderr, pullErr := client.Run(pullCmd)
			if pullErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("镜像拉取失败: %s", pullStderr))
			}

			cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d", remoteDir)
			stdout, stderr, err := client.Run(cmd)
			if err != nil {
				result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
				result.Output = stdout + "\n" + stderr
				return result, nil
			}
			result.Output = stdout
		}
	}

	result.Success = true
	return result, nil
}

func (h *ServiceHandler) getRequiredNetworks(change *valueobject.Change, deps DepsProvider, serverName string) ([]entity.ServerNetwork, error) {
	server, ok := deps.Server(serverName)
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	var serviceNetworks []string
	if change.NewState != nil {
		if svc, ok := change.NewState.(*entity.BizService); ok {
			serviceNetworks = svc.Networks
		} else if svc, ok := change.NewState.(map[string]interface{}); ok {
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
		if server.HasNetwork(netName) {
			requiredNetworks = append(requiredNetworks, *server.GetNetwork(netName))
		} else {
			requiredNetworks = append(requiredNetworks, entity.ServerNetwork{Name: netName, Type: entity.NetworkTypeBridge})
		}
	}
	return requiredNetworks, nil
}

func (h *ServiceHandler) deleteService(change *valueobject.Change, client SSHClient, remoteDir string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml down -v 2>/dev/null || true", remoteDir)
	stdout, stderr, _ := client.Run(cmd)

	rmCmd := fmt.Sprintf("sudo rm -rf %s", remoteDir)
	stdout2, stderr2, err := client.Run(rmCmd)
	if err != nil {
		result.Error = fmt.Errorf("failed to remove directory: %w, stderr: %s", err, stderr2)
		result.Output = stdout + "\n" + stderr + "\n" + stdout2 + "\n" + stderr2
		return result, nil
	}

	result.Success = true
	result.Output = stdout + "\n" + stdout2
	return result, nil
}

func (h *ServiceHandler) getComposeFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, ch.Name+".compose.yaml")
}
