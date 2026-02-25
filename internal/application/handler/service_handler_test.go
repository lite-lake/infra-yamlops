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
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "myapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name:  "myapp",
			Image: "nginx:latest",
		})

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

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "service", "myapp").
		WithOldState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "myapp",
		})

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

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "myapp").
		WithNewState(&entity.BizService{
			Name: "myapp",
		})

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

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "myapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "myapp",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for SSH error")
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

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "myapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "myapp",
		})

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

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "myapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "myapp",
		})

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

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "service", "myapp").
		WithOldState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "myapp",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for remove error")
	}
}

func TestServiceHandler_GetComposeFilePath(t *testing.T) {
	tests := []struct {
		name     string
		change   *valueobject.Change
		workDir  string
		expected string
	}{
		{
			name: "valid path",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "myapp").
				WithNewState(&entity.BizService{
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Name: "myapp",
				}),
			workDir:  "/tmp",
			expected: filepath.Join("/tmp", "deployments", "server1", "myapp.compose.yaml"),
		},
		{
			name: "no server in state",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "myapp").
				WithNewState(&entity.BizService{
					Name: "myapp",
				}),
			workDir:  "/tmp",
			expected: "",
		},
		{
			name: "server from old state",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "myapp").
				WithOldState(&entity.BizService{
					ServiceBase: entity.ServiceBase{
						Server: "server2",
					},
					Name: "myapp",
				}),
			workDir:  "/opt",
			expected: filepath.Join("/opt", "deployments", "server2", "myapp.compose.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newMockDeps()
			deps.workDir = tt.workDir
			result := GetComposeFilePath(tt.change, deps)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestServiceHandler_DeployService_WithComposeFile(t *testing.T) {
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
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "prod"
	deps.workDir = tmpDir

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "testapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "testapp",
		})

	deployCtx := &ServiceDeployContext{
		ServerName: "server1",
		Client:     mockSSH,
		RemoteDir:  "/opt/yamlops/yo-prod-testapp",
	}
	result, err := ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{
		RestartAfterUp: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestServiceHandler_DeployService_ReadFileError(t *testing.T) {
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
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = tmpDir

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "service", "testapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "testapp",
		})

	os.Remove(composeFile)

	deployCtx := &ServiceDeployContext{
		ServerName: "server1",
		Client:     mockSSH,
		RemoteDir:  "/opt/test",
	}
	result, err := ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{
		RestartAfterUp: false,
	})
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
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "service", "myapp").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name:  "myapp",
			Image: "nginx:latest",
		})

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
			name: "server from new state",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").
				WithNewState(&entity.BizService{
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Name: "myapp",
				}),
			expected: "server1",
		},
		{
			name: "server from old state",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").
				WithOldState(&entity.BizService{
					ServiceBase: entity.ServiceBase{
						Server: "server2",
					},
					Name: "myapp",
				}),
			expected: "server2",
		},
		{
			name: "prefer old state over new",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").
				WithOldState(&entity.BizService{
					ServiceBase: entity.ServiceBase{
						Server: "old-server",
					},
					Name: "myapp",
				}).
				WithNewState(&entity.BizService{
					ServiceBase: entity.ServiceBase{
						Server: "new-server",
					},
					Name: "myapp",
				}),
			expected: "old-server",
		},
		{
			name:     "no server in state",
			change:   valueobject.NewChange(valueobject.ChangeTypeNoop, "", ""),
			expected: "",
		},
		{
			name: "server from map[string]interface{}",
			change: valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").
				WithNewState(map[string]interface{}{
					"server": "server3",
				}),
			expected: "server3",
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
