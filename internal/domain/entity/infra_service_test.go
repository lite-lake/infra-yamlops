package entity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

func TestGatewayPorts_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ports   GatewayPorts
		wantErr error
	}{
		{
			name:    "invalid http port zero",
			ports:   GatewayPorts{HTTP: 0, HTTPS: 443},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid http port negative",
			ports:   GatewayPorts{HTTP: -1, HTTPS: 443},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid http port too large",
			ports:   GatewayPorts{HTTP: 65536, HTTPS: 443},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid https port zero",
			ports:   GatewayPorts{HTTP: 80, HTTPS: 0},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid https port too large",
			ports:   GatewayPorts{HTTP: 80, HTTPS: 70000},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "valid",
			ports:   GatewayPorts{HTTP: 80, HTTPS: 443},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ports.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestGatewaySSLConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  GatewaySSLConfig
		wantErr error
	}{
		{
			name:    "invalid mode",
			config:  GatewaySSLConfig{Mode: "invalid"},
			wantErr: domain.ErrInvalidType,
		},
		{
			name:    "remote mode without endpoint",
			config:  GatewaySSLConfig{Mode: "remote"},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "valid local mode",
			config:  GatewaySSLConfig{Mode: "local"},
			wantErr: nil,
		},
		{
			name:    "valid remote mode",
			config:  GatewaySSLConfig{Mode: "remote", Endpoint: "ssl-server:8080"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestGatewayWAFConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  GatewayWAFConfig
		wantErr error
	}{
		{
			name:    "invalid cidr",
			config:  GatewayWAFConfig{Enabled: true, Whitelist: []string{"invalid"}},
			wantErr: domain.ErrInvalidCIDR,
		},
		{
			name:    "valid empty whitelist",
			config:  GatewayWAFConfig{Enabled: true},
			wantErr: nil,
		},
		{
			name:    "valid with whitelist",
			config:  GatewayWAFConfig{Enabled: true, Whitelist: []string{"192.168.1.0/24", "10.0.0.0/8"}},
			wantErr: nil,
		},
		{
			name:    "valid default_mode block",
			config:  GatewayWAFConfig{Enabled: true, DefaultMode: "block"},
			wantErr: nil,
		},
		{
			name:    "valid default_mode detect",
			config:  GatewayWAFConfig{Enabled: true, DefaultMode: "detect"},
			wantErr: nil,
		},
		{
			name:    "valid default_mode disabled",
			config:  GatewayWAFConfig{Enabled: true, DefaultMode: "disabled"},
			wantErr: nil,
		},
		{
			name:    "invalid default_mode",
			config:  GatewayWAFConfig{Enabled: true, DefaultMode: "invalid"},
			wantErr: fmt.Errorf("waf.default_mode must be 'block', 'detect' or 'disabled', got 'invalid'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
				} else if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestInfraService_Validate(t *testing.T) {
	tests := []struct {
		name    string
		service InfraService
		wantErr error
	}{
		{
			name:    "missing name",
			service: InfraService{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "invalid type",
			service: InfraService{Name: "gateway-1", Type: "invalid"},
			wantErr: domain.ErrInvalidType,
		},
		{
			name:    "missing server",
			service: InfraService{Name: "gateway-1", Type: InfraServiceTypeGateway},
			wantErr: domain.ErrRequired,
		},
		{
			name: "missing image",
			service: InfraService{
				Name: "gateway-1",
				Type: InfraServiceTypeGateway,
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
			},
			wantErr: domain.ErrRequired,
		},
		{
			name: "gateway invalid ports",
			service: InfraService{
				Name: "gateway-1",
				Type: InfraServiceTypeGateway,
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image:        "nginx:latest",
				GatewayPorts: &GatewayPorts{HTTP: 0, HTTPS: 443},
			},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name: "valid gateway",
			service: InfraService{
				Name: "gateway-1",
				Type: InfraServiceTypeGateway,
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image:        "nginx:latest",
				GatewayPorts: &GatewayPorts{HTTP: 80, HTTPS: 443},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.service.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestInfraService_GetServer(t *testing.T) {
	s := InfraService{ServiceBase: ServiceBase{Server: "my-server"}}
	if got := s.GetServer(); got != "my-server" {
		t.Errorf("GetServer() = %v, want my-server", got)
	}
}
