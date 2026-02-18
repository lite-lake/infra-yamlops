package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type GatewayHandler struct{}

func NewGatewayHandler() *GatewayHandler {
	return &GatewayHandler{}
}

func (h *GatewayHandler) EntityType() string {
	return "gateway"
}

func (h *GatewayHandler) Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error) {
	result := &Result{Change: change, Success: false}

	serverName := ExtractServerFromChange(change)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", change.Name)
		return result, nil
	}

	remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", deps.Env, change.Name)

	if change.Type == valueobject.ChangeTypeDelete {
		return h.deleteGateway(change, deps, remoteDir)
	}

	return h.deployGateway(change, deps, serverName, remoteDir)
}

func (h *GatewayHandler) deployGateway(change *valueobject.Change, deps *Deps, serverName, remoteDir string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if deps.SSHClient == nil {
		result.Error = fmt.Errorf("SSH client not available")
		return result, nil
	}

	if err := deps.SSHClient.MkdirAllSudoWithPerm(remoteDir, "750"); err != nil {
		result.Error = fmt.Errorf("failed to create remote directory: %w", err)
		return result, nil
	}

	gatewayFile := h.getGatewayFilePath(change, deps)
	if gatewayFile == "" {
		result.Success = true
		result.Output = "no gateway file to deploy"
		return result, nil
	}

	if _, err := os.Stat(gatewayFile); os.IsNotExist(err) {
		result.Success = true
		result.Output = "gateway file not found, skipping"
		return result, nil
	}

	content, err := os.ReadFile(gatewayFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to read gateway file: %w", err)
		return result, nil
	}

	client, ok := deps.Servers[serverName]
	if !ok {
		result.Error = fmt.Errorf("server %s not found", serverName)
		return result, nil
	}
	_ = client

	if deps.SSHClient == nil {
		result.Error = fmt.Errorf("SSH client not available")
		return result, nil
	}

	if err := SyncContent(deps.SSHClient, string(content), remoteDir+"/gateway.yml"); err != nil {
		result.Error = fmt.Errorf("failed to sync gateway file: %w", err)
		return result, nil
	}

	composeFile := h.getComposeFilePath(change, deps)
	if composeFile != "" {
		if _, err := os.Stat(composeFile); err == nil {
			composeContent, err := os.ReadFile(composeFile)
			if err != nil {
				result.Error = fmt.Errorf("failed to read compose file: %w", err)
				return result, nil
			}
			if err := SyncContent(deps.SSHClient, string(composeContent), remoteDir+"/docker-compose.yml"); err != nil {
				result.Error = fmt.Errorf("failed to sync compose file: %w", err)
				return result, nil
			}

			networkCmd := fmt.Sprintf("sudo docker network create yamlops-%s 2>/dev/null || true", deps.Env)
			_, _, _ = deps.SSHClient.Run(networkCmd)

			containerName := fmt.Sprintf("yo-%s-%s", deps.Env, change.Name)
			cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d && sudo docker restart %s", remoteDir, containerName)
			stdout, stderr, err := deps.SSHClient.Run(cmd)
			if err != nil {
				result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
				result.Output = stdout + "\n" + stderr
				return result, nil
			}
		}
	}

	result.Success = true
	result.Output = fmt.Sprintf("deployed gateway %s", change.Name)
	return result, nil
}

func (h *GatewayHandler) deleteGateway(change *valueobject.Change, deps *Deps, remoteDir string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if deps.SSHClient == nil {
		result.Error = fmt.Errorf("SSH client not available")
		return result, nil
	}

	rmCmd := fmt.Sprintf("sudo rm -f %s/gateway.yml", remoteDir)
	stdout, stderr, err := deps.SSHClient.Run(rmCmd)
	if err != nil {
		result.Error = fmt.Errorf("failed to remove gateway file: %w, stderr: %s", err, stderr)
		result.Output = stdout + "\n" + stderr
		return result, nil
	}

	result.Success = true
	result.Output = fmt.Sprintf("removed gateway %s", change.Name)
	return result, nil
}

func (h *GatewayHandler) getGatewayFilePath(ch *valueobject.Change, deps *Deps) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir, "deployments", serverName, ch.Name+".gate.yaml")
}

func (h *GatewayHandler) getComposeFilePath(ch *valueobject.Change, deps *Deps) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir, "deployments", serverName, ch.Name+".compose.yaml")
}
