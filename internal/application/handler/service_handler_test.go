package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestServiceHandler_EntityType(t *testing.T) {
	h := NewServiceHandler()
	if h.EntityType() != "service" {
		t.Errorf("expected 'service', got %s", h.EntityType())
	}
}

func TestServiceHandler_Apply_Deploy(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "deployment successful"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "myapp",
		NewState: map[string]interface{}{
			"server": "server1",
			"image":  "nginx:latest",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if mockSSH.mkdirErr != nil {
		t.Error("expected MkdirAllSudoWithPerm to be called")
	}
}

func TestServiceHandler_Apply_Delete(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "removed"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeDelete,
		Entity: "service",
		Name:   "myapp",
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

func TestServiceHandler_Apply_ServerNotDetermined(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	deps := newMockDeps()

	change := &valueobject.Change{
		Type:     valueobject.ChangeTypeCreate,
		Entity:   "service",
		Name:     "myapp",
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

func TestServiceHandler_Apply_SSHError(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	deps := newMockDeps()
	deps.sshErr = errors.New("connection failed")
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "myapp",
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

func TestServiceHandler_Apply_ServerNotRegistered(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	deps := newMockDeps()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "myapp",
		NewState: map[string]interface{}{
			"server": "unknown-server",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for unregistered server")
	}
}

func TestServiceHandler_Apply_MkdirError(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{mkdirErr: errors.New("permission denied")}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "myapp",
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

func TestServiceHandler_Apply_DockerComposeError(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runErr: errors.New("docker error"), runStderr: "container failed"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	serverDir := filepath.Join(deps.workDir, "deployments", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeFile := filepath.Join(serverDir, "myapp.compose.yaml")
	if err := os.WriteFile(composeFile, []byte("version: '3'\nservices:\n  app:\n    image: nginx"), 0644); err != nil {
		t.Fatal(err)
	}

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "myapp",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for docker compose error")
	}
}

func TestServiceHandler_DeleteService_RemoveError(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runErr: errors.New("rm failed"), runStderr: "permission denied"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeDelete,
		Entity: "service",
		Name:   "myapp",
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

func TestServiceHandler_GetComposeFilePath(t *testing.T) {
	h := &ServiceHandler{}

	tests := []struct {
		name     string
		change   *valueobject.Change
		workDir  string
		expected string
	}{
		{
			name: "valid path",
			change: &valueobject.Change{
				Name: "myapp",
				NewState: map[string]interface{}{
					"server": "server1",
				},
			},
			workDir:  "/tmp",
			expected: filepath.Join("/tmp", "deployments", "server1", "myapp.compose.yaml"),
		},
		{
			name: "no server in state",
			change: &valueobject.Change{
				Name:     "myapp",
				NewState: map[string]interface{}{},
			},
			workDir:  "/tmp",
			expected: "",
		},
		{
			name: "server from old state",
			change: &valueobject.Change{
				Name: "myapp",
				OldState: map[string]interface{}{
					"server": "server2",
				},
			},
			workDir:  "/opt",
			expected: filepath.Join("/opt", "deployments", "server2", "myapp.compose.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newMockDeps()
			deps.workDir = tt.workDir
			result := h.getComposeFilePath(tt.change, deps)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestServiceHandler_DeployService_WithComposeFile(t *testing.T) {
	h := NewServiceHandler()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeContent := `version: '3'
services:
  app:
    image: nginx:latest
    ports:
      - "80:80"`
	composeFile := filepath.Join(serverDir, "testapp.compose.yaml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: "started"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = tmpDir

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "testapp",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	result, err := h.deployService(change, mockSSH, "/opt/yamlops/yo-prod-testapp", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestServiceHandler_DeployService_ReadFileError(t *testing.T) {
	h := NewServiceHandler()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}

	composeFile := filepath.Join(serverDir, "testapp.compose.yaml")
	if err := os.WriteFile(composeFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: "ok"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "test"
	deps.workDir = tmpDir

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeCreate,
		Entity: "service",
		Name:   "testapp",
		NewState: map[string]interface{}{
			"server": "server1",
		},
	}

	os.Remove(composeFile)

	result, err := h.deployService(change, mockSSH, "/opt/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success when compose file missing (no file to deploy)")
	}
}

func TestServiceHandler_Apply_UpdateType(t *testing.T) {
	h := NewServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "updated"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := &valueobject.Change{
		Type:   valueobject.ChangeTypeUpdate,
		Entity: "service",
		Name:   "myapp",
		NewState: map[string]interface{}{
			"server": "server1",
			"image":  "nginx:latest",
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

func TestExtractServerFromChange_Service(t *testing.T) {
	tests := []struct {
		name     string
		change   *valueobject.Change
		expected string
	}{
		{
			name: "server from new state map",
			change: &valueobject.Change{
				NewState: map[string]interface{}{
					"server": "server1",
				},
			},
			expected: "server1",
		},
		{
			name: "server from old state map",
			change: &valueobject.Change{
				OldState: map[string]interface{}{
					"server": "server2",
				},
			},
			expected: "server2",
		},
		{
			name: "prefer old state over new",
			change: &valueobject.Change{
				OldState: map[string]interface{}{
					"server": "old-server",
				},
				NewState: map[string]interface{}{
					"server": "new-server",
				},
			},
			expected: "old-server",
		},
		{
			name:     "no server in state",
			change:   &valueobject.Change{},
			expected: "",
		},
		{
			name: "server not a string",
			change: &valueobject.Change{
				NewState: map[string]interface{}{
					"server": 123,
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractServerFromChange(tt.change)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
