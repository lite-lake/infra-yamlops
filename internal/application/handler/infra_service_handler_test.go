package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestInfraServiceHandler_EntityType(t *testing.T) {
	h := NewInfraServiceHandler()
	if h.EntityType() != "infra_service" {
		t.Errorf("expected 'infra_service', got %s", h.EntityType())
	}
}

func TestInfraServiceHandler_Apply_ServerNotDetermined(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()
	deps := newMockDeps()

	change := &valueobject.Change{
		Type:     valueobject.ChangeTypeCreate,
		Entity:   "infra_service",
		Name:     "gateway1",
		NewState: map[string]interface{}{},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when server cannot be determined")
	}
}

func TestInfraServiceHandler_Apply_ServerNotRegistered(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()
	deps := newMockDeps()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: map[string]interface{}{
			"server": "unknown",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when server not registered")
	}
}

func TestInfraServiceHandler_Apply_SSHError(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	deps := newMockDeps()
	deps.sshErr = errors.New("connection failed")
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for SSH error")
	}
}

func TestInfraServiceHandler_Apply_MkdirError(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{mkdirErr: errors.New("permission denied")}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for mkdir error")
	}
}

func TestInfraServiceHandler_Apply_Deploy(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "deployed"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: &entity.InfraService{
			Name:   "gateway1",
			Server: "server1",
			Type:   entity.InfraServiceTypeGateway,
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_Apply_DeploySSL(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "deployed"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "ssl1",
		NewState: &entity.InfraService{
			Name:   "ssl1",
			Server: "server1",
			Type:   entity.InfraServiceTypeSSL,
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_Apply_Delete(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "removed"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeDelete,
		Entity: "infra_service",
		Name:   "gateway1",
		OldState: map[string]interface{}{
			"server": "server1",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_Delete_RemoveError(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runErr: errors.New("rm failed"), runStderr: "permission denied"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeDelete,
		Entity: "infra_service",
		Name:   "gateway1",
		OldState: map[string]interface{}{
			"server": "server1",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for remove error")
	}
}

func TestInfraServiceHandler_DeployInfraService_InvalidState(t *testing.T) {
	h := NewInfraServiceHandler()

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:     valueobject.ChangeTypeCreate,
		Entity:   "infra_service",
		Name:     "gateway1",
		NewState: "invalid state",
	}

	result, err := h.deployInfraService(change, mockSSH, "/opt/test", deps, "server1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for invalid state")
	}
}

func TestInfraServiceHandler_DeployInfraService_WithComposeFile(t *testing.T) {
	h := NewInfraServiceHandler()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeContent := `version: '3'
services:
  app:
    image: nginx:latest`
	composeFile := filepath.Join(serverDir, "gateway1.compose.yaml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: "started"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "prod"
	deps.workDir = tmpDir

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: &entity.InfraService{
			Name:   "gateway1",
			Server: "server1",
			Type:   entity.InfraServiceTypeGateway,
		},
	}

	result, err := h.deployInfraService(change, mockSSH, "/opt/test", deps, "server1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_DeployGatewayType_WithFile(t *testing.T) {
	h := NewInfraServiceHandler()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	gatewayFile := filepath.Join(serverDir, "gateway1.gate.yaml")
	if err := os.WriteFile(gatewayFile, []byte("gateway config"), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = tmpDir

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: &entity.InfraService{
			Name:   "gateway1",
			Server: "server1",
		},
	}

	err := h.deployGatewayType(change, mockSSH, "/opt/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockSSH.uploaded) == 0 {
		t.Error("expected file to be uploaded")
	}
}

func TestInfraServiceHandler_DeploySSLType_WithFile(t *testing.T) {
	h := NewInfraServiceHandler()

	tmpDir := t.TempDir()
	sslConfigDir := filepath.Join(tmpDir, "deployments", "server1", "ssl-config")
	if err := os.MkdirAll(sslConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	sslConfigFile := filepath.Join(sslConfigDir, "config.yml")
	if err := os.WriteFile(sslConfigFile, []byte("ssl config"), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = tmpDir

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "ssl1",
		NewState: &entity.InfraService{
			Name:   "ssl1",
			Server: "server1",
			Type:   entity.InfraServiceTypeSSL,
		},
	}

	err := h.deploySSLType(change, mockSSH, "/opt/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfraServiceHandler_GetComposeFilePath(t *testing.T) {
	h := &InfraServiceHandler{}

	deps := newMockDeps()
	deps.workDir = "/tmp"
	deps.servers["server1"] = &ServerInfo{}

	change := &valueobject.Change{
		Name: "gateway1",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	result := h.getComposeFilePath(change, deps)
	expected := filepath.Join("/tmp", "deployments", "server1", "gateway1.compose.yaml")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInfraServiceHandler_GetGatewayFilePath(t *testing.T) {
	h := &InfraServiceHandler{}

	deps := newMockDeps()
	deps.workDir = "/tmp"
	deps.servers["server1"] = &ServerInfo{}

	change := &valueobject.Change{
		Name: "gateway1",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	result := h.getGatewayFilePath(change, deps)
	expected := filepath.Join("/tmp", "deployments", "server1", "gateway1.gate.yaml")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInfraServiceHandler_GetSSLConfigFilePath(t *testing.T) {
	h := &InfraServiceHandler{}

	deps := newMockDeps()
	deps.workDir = "/tmp"
	deps.env = "demo"

	infra := &entity.InfraService{
		Name: "ssl1",
		Type: entity.InfraServiceTypeSSL,
		SSLConfig: &entity.SSLConfig{
			Config: &entity.SSLVolumeConfig{
				Source: "volumes://infra-ssl-config-cn",
				Sync:   true,
			},
		},
	}

	result := h.getSSLConfigFilePath(infra, deps)
	expected := filepath.Join("/tmp", "userdata", "demo", "volumes", "infra-ssl-config-cn", "config.yml")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInfraServiceHandler_DeployGatewayType_NoFile(t *testing.T) {
	h := NewInfraServiceHandler()

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = t.TempDir()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "gateway1",
		NewState: &entity.InfraService{
			Name:   "gateway1",
			Server: "server1",
		},
	}

	err := h.deployGatewayType(change, mockSSH, "/opt/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfraServiceHandler_DeploySSLType_NoFile(t *testing.T) {
	h := NewInfraServiceHandler()

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = t.TempDir()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "infra_service",
		Name:   "ssl1",
		NewState: &entity.InfraService{
			Name:   "ssl1",
			Server: "server1",
		},
	}

	err := h.deploySSLType(change, mockSSH, "/opt/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
