package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	remoteDir := GetRemoteDir(deps, change.Name)

	if change.Type == valueobject.ChangeTypeDelete {
		return DeleteServiceRemote(change, client, remoteDir)
	}

	return h.deployInfraService(change, client, remoteDir, deps, serverName)
}

func (h *InfraServiceHandler) deployInfraService(change *valueobject.Change, client SSHClient, remoteDir string, deps DepsProvider, serverName string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	infra, ok := change.NewState.(*entity.InfraService)
	if !ok && change.NewState != nil {
		result.Error = fmt.Errorf("invalid infra service state")
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

	composeFile := GetComposeFilePath(change, deps)
	if !DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:      remoteDir,
		ComposeFile:    composeFile,
		Env:            deps.Env(),
		ServiceName:    change.Name,
		RestartAfterUp: true,
	}, result) {
		return result, nil
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
	infra, ok := change.NewState.(*entity.InfraService)
	if !ok || infra == nil || infra.SSLConfig == nil || infra.SSLConfig.Config == nil {
		return nil
	}

	sslConfigFile := h.getSSLConfigFilePath(infra, deps)
	if sslConfigFile == "" {
		return nil
	}

	if _, err := os.Stat(sslConfigFile); os.IsNotExist(err) {
		return fmt.Errorf("ssl config file not found: %s", sslConfigFile)
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

func (h *InfraServiceHandler) getGatewayFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, ch.Name+".gate.yaml")
}

func (h *InfraServiceHandler) getSSLConfigFilePath(infra *entity.InfraService, deps DepsProvider) string {
	if infra.SSLConfig == nil || infra.SSLConfig.Config == nil || infra.SSLConfig.Config.Source == "" {
		return ""
	}

	source := infra.SSLConfig.Config.Source
	if !strings.HasPrefix(source, "volumes://") {
		return ""
	}

	volumePath := strings.TrimPrefix(source, "volumes://")
	return filepath.Join(deps.WorkDir(), "userdata", deps.Env(), "volumes", volumePath, "config.yml")
}
