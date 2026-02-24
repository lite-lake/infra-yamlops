package handler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/network"
)

type NetworkGetter interface {
	GetNetworks() []string
}

func GetRequiredNetworks(change *valueobject.Change, deps DepsProvider, serverName string) ([]entity.ServerNetwork, error) {
	server, ok := deps.Server(serverName)
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	var serviceNetworks []string
	if change.NewState != nil {
		if getter, ok := change.NewState.(NetworkGetter); ok {
			serviceNetworks = getter.GetNetworks()
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

func EnsureNetworks(client SSHClient, networks []entity.ServerNetwork) error {
	if len(networks) == 0 {
		return nil
	}
	netMgr := network.NewManager(client)
	for _, netSpec := range networks {
		if err := netMgr.Ensure(&netSpec); err != nil {
			return fmt.Errorf("failed to ensure network %s: %w", netSpec.Name, err)
		}
	}
	return nil
}

func DeleteServiceRemote(change *valueobject.Change, client SSHClient, remoteDir string) (*Result, error) {
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

func GetComposeFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, ch.Name+".compose.yaml")
}

type DeployComposeConfig struct {
	RemoteDir      string
	ComposeFile    string
	Env            string
	ServiceName    string
	RestartAfterUp bool
}

func DeployComposeFile(client SSHClient, cfg *DeployComposeConfig, result *Result) bool {
	if cfg.ComposeFile == "" {
		return true
	}

	if _, err := os.Stat(cfg.ComposeFile); err != nil {
		return true
	}

	content, err := os.ReadFile(cfg.ComposeFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to read compose file: %w", err)
		return false
	}
	if err := SyncContent(client, string(content), cfg.RemoteDir+"/docker-compose.yml"); err != nil {
		result.Error = fmt.Errorf("failed to sync compose file: %w", err)
		return false
	}

	pullCmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml pull", cfg.RemoteDir)
	_, pullStderr, pullErr := client.Run(pullCmd)
	if pullErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("镜像拉取失败: %s", pullStderr))
	}

	var cmd string
	if cfg.RestartAfterUp {
		containerName := fmt.Sprintf(constants.ServicePrefixFormat, cfg.Env, cfg.ServiceName)
		cmd = fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d && sudo docker restart %s", cfg.RemoteDir, containerName)
	} else {
		cmd = fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d", cfg.RemoteDir)
	}

	stdout, stderr, err := client.Run(cmd)
	if err != nil {
		result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
		result.Output = stdout + "\n" + stderr
		return false
	}
	result.Output = stdout
	return true
}

func GetRemoteDir(deps DepsProvider, serviceName string) string {
	return fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, deps.Env(), serviceName))
}

func EnsureRemoteDir(client SSHClient, remoteDir string) error {
	return client.MkdirAllSudoWithPerm(remoteDir, "755")
}
