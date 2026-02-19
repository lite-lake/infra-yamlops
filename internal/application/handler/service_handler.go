package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type ServiceHandler struct{}

func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

func (h *ServiceHandler) EntityType() string {
	return "service"
}

func (h *ServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error) {
	result := &Result{Change: change, Success: false}

	serverName := ExtractServerFromChange(change)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", change.Name)
		return result, nil
	}

	client, err := h.getClient(serverName, deps)
	if err != nil {
		result.Error = fmt.Errorf("failed to get SSH client: %w", err)
		return result, nil
	}

	remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", deps.Env, change.Name)

	switch change.Type {
	case valueobject.ChangeTypeDelete:
		return h.deleteService(change, client, remoteDir, deps)
	default:
		return h.deployService(change, client, remoteDir, deps)
	}
}

func (h *ServiceHandler) getClient(serverName string, deps *Deps) (SSHClient, error) {
	if _, ok := deps.Servers[serverName]; !ok {
		return nil, fmt.Errorf("server %s not registered", serverName)
	}
	if deps.SSHClient == nil {
		errMsg := fmt.Sprintf("SSH client not available for server %s", serverName)
		if deps.SSHError != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, deps.SSHError)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	return deps.SSHClient, nil
}

func (h *ServiceHandler) deployService(change *valueobject.Change, client SSHClient, remoteDir string, deps *Deps) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if err := client.MkdirAllSudoWithPerm(remoteDir, "750"); err != nil {
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

			networkCmd := fmt.Sprintf("sudo docker network create yamlops-%s 2>/dev/null || true", deps.Env)
			_, _, _ = client.Run(networkCmd)

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

func (h *ServiceHandler) deleteService(change *valueobject.Change, client SSHClient, remoteDir string, deps *Deps) (*Result, error) {
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

func (h *ServiceHandler) getComposeFilePath(ch *valueobject.Change, deps *Deps) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir, "deployments", serverName, ch.Name+".compose.yaml")
}
