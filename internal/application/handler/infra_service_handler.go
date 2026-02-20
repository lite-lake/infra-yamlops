package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type InfraServiceHandler struct{}

func NewInfraServiceHandler() *InfraServiceHandler {
	return &InfraServiceHandler{}
}

func (h *InfraServiceHandler) EntityType() string {
	return "infra_service"
}

func (h *InfraServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	serverName := ExtractServerFromChange(change)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", change.Name)
		return result, nil
	}

	if _, ok := deps.ServerInfo(serverName); !ok {
		result.Error = fmt.Errorf("server %s not registered", serverName)
		return result, nil
	}

	client, err := deps.SSHClient(serverName)
	if err != nil {
		result.Error = err
		return result, nil
	}

	remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, deps.Env(), change.Name))

	if change.Type == valueobject.ChangeTypeDelete {
		return h.deleteInfraService(change, client, remoteDir)
	}

	return h.deployInfraService(change, client, remoteDir, deps)
}

func (h *InfraServiceHandler) deployInfraService(change *valueobject.Change, client SSHClient, remoteDir string, deps DepsProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	infra, ok := change.NewState.(*entity.InfraService)
	if !ok && change.NewState != nil {
		result.Error = fmt.Errorf("invalid infra service state")
		return result, nil
	}

	if err := client.MkdirAllSudoWithPerm(remoteDir, "750"); err != nil {
		result.Error = fmt.Errorf("failed to create remote directory: %w", err)
		return result, nil
	}

	if infra != nil && infra.Type == entity.InfraServiceTypeGateway {
		if err := h.deployGatewayType(change, client, remoteDir, deps); err != nil {
			result.Error = err
			return result, nil
		}
	}

	if infra != nil && infra.Type == entity.InfraServiceTypeSSL {
		if err := h.deploySSLType(change, client, remoteDir, deps); err != nil {
			result.Error = err
			return result, nil
		}
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

			networkCmd := fmt.Sprintf("sudo docker network create %s 2>/dev/null || true", fmt.Sprintf(constants.DockerNetworkFormat, deps.Env()))
			_, _, _ = client.Run(networkCmd)

			pullCmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml pull", remoteDir)
			_, pullStderr, pullErr := client.Run(pullCmd)
			if pullErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("镜像拉取失败: %s", pullStderr))
			}

			containerName := fmt.Sprintf(constants.ServicePrefixFormat, deps.Env(), change.Name)
			cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d && sudo docker restart %s", remoteDir, containerName)
			stdout, stderr, err := client.Run(cmd)
			if err != nil {
				result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
				result.Output = stdout + "\n" + stderr
				return result, nil
			}
		}
	}

	result.Success = true
	result.Output = fmt.Sprintf("deployed infra service %s", change.Name)
	return result, nil
}

func (h *InfraServiceHandler) deployGatewayType(change *valueobject.Change, client SSHClient, remoteDir string, deps DepsProvider) error {
	gatewayFile := h.getGatewayFilePath(change, deps)
	if gatewayFile == "" {
		return nil
	}

	if _, err := os.Stat(gatewayFile); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(gatewayFile)
	if err != nil {
		return fmt.Errorf("failed to read gateway file: %w", err)
	}

	if err := SyncContent(client, string(content), remoteDir+"/gateway.yml"); err != nil {
		return fmt.Errorf("failed to sync gateway file: %w", err)
	}

	return nil
}

func (h *InfraServiceHandler) deploySSLType(change *valueobject.Change, client SSHClient, remoteDir string, deps DepsProvider) error {
	sslConfigFile := h.getSSLConfigFilePath(change, deps)
	if sslConfigFile == "" {
		return nil
	}

	if _, err := os.Stat(sslConfigFile); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(sslConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read ssl config file: %w", err)
	}

	if err := client.MkdirAllSudoWithPerm(remoteDir+"/ssl-config", "755"); err != nil {
		return fmt.Errorf("failed to create ssl-config directory: %w", err)
	}

	if err := SyncContent(client, string(content), remoteDir+"/ssl-config/config.yml"); err != nil {
		return fmt.Errorf("failed to sync ssl config file: %w", err)
	}

	return nil
}

func (h *InfraServiceHandler) deleteInfraService(change *valueobject.Change, client SSHClient, remoteDir string) (*Result, error) {
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
	result.Output = fmt.Sprintf("deleted infra service %s", change.Name)
	return result, nil
}

func (h *InfraServiceHandler) getComposeFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, ch.Name+".compose.yaml")
}

func (h *InfraServiceHandler) getGatewayFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, ch.Name+".gate.yaml")
}

func (h *InfraServiceHandler) getSSLConfigFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, "ssl-config", "config.yml")
}
