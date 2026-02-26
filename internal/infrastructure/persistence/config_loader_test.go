package persistence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/service"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func TestConfigLoader_Load(t *testing.T) {
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, "userdata", "test")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := loader.Load(context.Background(), "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent directory")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		cfg, err := loader.Load(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
	})
}

func TestConfigLoader_Validate(t *testing.T) {
	loader := NewConfigLoader(".")

	t.Run("nil config", func(t *testing.T) {
		err := loader.Validate(nil)
		if err != domain.ErrConfigNotLoaded {
			t.Errorf("expected ErrConfigNotLoaded, got %v", err)
		}
	})

	t.Run("empty config", func(t *testing.T) {
		cfg := &entity.Config{}
		err := loader.Validate(cfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoadEntity(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.yaml")

	tests := []struct {
		name    string
		content string
		yamlKey string
		wantLen int
		wantErr bool
	}{
		{
			name: "valid secrets",
			content: `secrets:
  - name: test
    value: secret123
`,
			yamlKey: "secrets",
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "valid isps",
			content: `isps:
  - name: isp1
    services:
      - server
    credentials:
      api_key: test
`,
			yamlKey: "isps",
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "valid zones",
			content: `zones:
  - name: zone1
    isp: isp1
    region: us-east-1
`,
			yamlKey: "zones",
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "valid servers",
			content: `servers:
  - name: server1
    zone: zone1
    isp: isp1
`,
			yamlKey: "servers",
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "empty file",
			content: ``,
			yamlKey: "items",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "missing key",
			content: `other:
  - name: test
`,
			yamlKey: "items",
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			switch tt.yamlKey {
			case "secrets":
				items, err := loadEntity[entity.Secret](tmpFile, tt.yamlKey)
				if (err != nil) != tt.wantErr {
					t.Errorf("loadEntity() error = %v, wantErr %v", err, tt.wantErr)
				}
				if len(items) != tt.wantLen {
					t.Errorf("loadEntity() got %d items, want %d", len(items), tt.wantLen)
				}
			case "isps":
				items, err := loadEntity[entity.ISP](tmpFile, tt.yamlKey)
				if (err != nil) != tt.wantErr {
					t.Errorf("loadEntity() error = %v, wantErr %v", err, tt.wantErr)
				}
				if len(items) != tt.wantLen {
					t.Errorf("loadEntity() got %d items, want %d", len(items), tt.wantLen)
				}
			case "zones":
				items, err := loadEntity[entity.Zone](tmpFile, tt.yamlKey)
				if (err != nil) != tt.wantErr {
					t.Errorf("loadEntity() error = %v, wantErr %v", err, tt.wantErr)
				}
				if len(items) != tt.wantLen {
					t.Errorf("loadEntity() got %d items, want %d", len(items), tt.wantLen)
				}
			case "servers":
				items, err := loadEntity[entity.Server](tmpFile, tt.yamlKey)
				if (err != nil) != tt.wantErr {
					t.Errorf("loadEntity() error = %v, wantErr %v", err, tt.wantErr)
				}
				if len(items) != tt.wantLen {
					t.Errorf("loadEntity() got %d items, want %d", len(items), tt.wantLen)
				}
			default:
				items, err := loadEntity[struct{}](tmpFile, tt.yamlKey)
				if (err != nil) != tt.wantErr {
					t.Errorf("loadEntity() error = %v, wantErr %v", err, tt.wantErr)
				}
				if len(items) != tt.wantLen {
					t.Errorf("loadEntity() got %d items, want %d", len(items), tt.wantLen)
				}
			}
		})
	}
}

func TestValidator_References(t *testing.T) {
	t.Run("valid references", func(t *testing.T) {
		cfg := &entity.Config{
			Secrets: []entity.Secret{{Name: "secret1", Value: "value1"}},
			ISPs:    []entity.ISP{{Name: "isp1", Services: []entity.ISPService{"server"}, Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefPlain("val")}}},
			Zones:   []entity.Zone{{Name: "zone1", ISP: "isp1", Region: "us-east-1"}},
		}
		validator := service.NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing isp reference", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", ISP: "nonexistent", Region: "us-east-1"}},
		}
		validator := service.NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "isp 'nonexistent' referenced by zone") {
			t.Errorf("expected missing isp error, got %v", err)
		}
	})
}

func TestValidator_PortConflicts(t *testing.T) {
	t.Run("no conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{
				Name: "srv1",
				Zone: "zone1",
				SSH:  entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")},
			}},
			InfraServices: []entity.InfraService{
				{Name: "ssl1", Type: entity.InfraServiceTypeSSL, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "nginx", SSLConfig: &entity.SSLConfig{Ports: entity.SSLPorts{API: 38567}, Config: &entity.SSLVolumeConfig{Source: "volumes://ssl", Sync: true}}},
			},
		}
		validator := service.NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{
				Name: "srv1",
				Zone: "zone1",
				SSH:  entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")},
			}},
			InfraServices: []entity.InfraService{
				{Name: "ssl1", Type: entity.InfraServiceTypeSSL, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "nginx", SSLConfig: &entity.SSLConfig{Ports: entity.SSLPorts{API: 38567}, Config: &entity.SSLVolumeConfig{Source: "volumes://ssl", Sync: true}}},
				{Name: "ssl2", Type: entity.InfraServiceTypeSSL, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "nginx", SSLConfig: &entity.SSLConfig{Ports: entity.SSLPorts{API: 38567}, Config: &entity.SSLVolumeConfig{Source: "volumes://ssl", Sync: true}}},
			},
		}
		validator := service.NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected error for port conflict, got %v", err)
		}
	})
}
