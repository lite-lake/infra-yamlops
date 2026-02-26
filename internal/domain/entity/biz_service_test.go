package entity

import (
	"errors"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func TestServiceHealthcheck_Validate(t *testing.T) {
	tests := []struct {
		name        string
		healthcheck ServiceHealthcheck
		wantErr     error
	}{
		{
			name:        "missing path",
			healthcheck: ServiceHealthcheck{},
			wantErr:     domain.ErrRequired,
		},
		{
			name:        "path without leading slash",
			healthcheck: ServiceHealthcheck{Path: "health"},
			wantErr:     domain.ErrInvalidPath,
		},
		{
			name:        "valid",
			healthcheck: ServiceHealthcheck{Path: "/health", Interval: "30s", Timeout: "5s"},
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.healthcheck.Validate()
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

func TestServiceVolume_Validate(t *testing.T) {
	tests := []struct {
		name    string
		volume  ServiceVolume
		wantErr error
	}{
		{
			name:    "missing source",
			volume:  ServiceVolume{Target: "/data"},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "missing target",
			volume:  ServiceVolume{Source: "/host"},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "valid",
			volume:  ServiceVolume{Source: "/host", Target: "/data", Sync: true},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.volume.Validate()
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

func TestServicePort_Validate(t *testing.T) {
	tests := []struct {
		name    string
		port    ServicePort
		wantErr error
	}{
		{
			name:    "invalid container port zero",
			port:    ServicePort{Container: 0, Host: 8080},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid container port negative",
			port:    ServicePort{Container: -1, Host: 8080},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid container port too large",
			port:    ServicePort{Container: 65536, Host: 8080},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid host port zero",
			port:    ServicePort{Container: 8080, Host: 0},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid host port too large",
			port:    ServicePort{Container: 8080, Host: 70000},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid protocol",
			port:    ServicePort{Container: 8080, Host: 80, Protocol: "http"},
			wantErr: domain.ErrInvalidProtocol,
		},
		{
			name:    "valid tcp protocol",
			port:    ServicePort{Container: 8080, Host: 80, Protocol: "tcp"},
			wantErr: nil,
		},
		{
			name:    "valid udp protocol",
			port:    ServicePort{Container: 8080, Host: 53, Protocol: "udp"},
			wantErr: nil,
		},
		{
			name:    "valid without protocol",
			port:    ServicePort{Container: 8080, Host: 80},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.port.Validate()
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

func TestServiceGatewayRoute_Validate(t *testing.T) {
	tests := []struct {
		name    string
		route   ServiceGatewayRoute
		wantErr error
	}{
		{
			name:    "missing hostname",
			route:   ServiceGatewayRoute{ContainerPort: 8080},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "invalid container port zero",
			route:   ServiceGatewayRoute{Hostname: "example.com", ContainerPort: 0},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid container port negative",
			route:   ServiceGatewayRoute{Hostname: "example.com", ContainerPort: -1},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid container port too large",
			route:   ServiceGatewayRoute{Hostname: "example.com", ContainerPort: 65536},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "valid",
			route:   ServiceGatewayRoute{Hostname: "example.com", ContainerPort: 8080, Path: "/api", HTTP: true},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.route.Validate()
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

func TestServiceGatewayRoute_HasGateway(t *testing.T) {
	tests := []struct {
		name     string
		route    ServiceGatewayRoute
		expected bool
	}{
		{"no gateway", ServiceGatewayRoute{}, false},
		{"http only", ServiceGatewayRoute{HTTP: true}, true},
		{"https only", ServiceGatewayRoute{HTTPS: true}, true},
		{"both", ServiceGatewayRoute{HTTP: true, HTTPS: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.route.HasGateway(); got != tt.expected {
				t.Errorf("HasGateway() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBizService_Validate(t *testing.T) {
	tests := []struct {
		name    string
		service BizService
		wantErr error
	}{
		{
			name:    "missing name",
			service: BizService{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "missing server",
			service: BizService{Name: "api"},
			wantErr: domain.ErrRequired,
		},
		{
			name: "missing image",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
			},
			wantErr: domain.ErrRequired,
		},
		{
			name: "invalid port",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image: "app:latest",
				Ports: []ServicePort{{Container: 0, Host: 80}},
			},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name: "invalid healthcheck",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image:       "app:latest",
				Healthcheck: &ServiceHealthcheck{Path: "invalid"},
			},
			wantErr: domain.ErrInvalidPath,
		},
		{
			name: "invalid volume",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image:   "app:latest",
				Volumes: []ServiceVolume{{Source: "", Target: "/data"}},
			},
			wantErr: domain.ErrRequired,
		},
		{
			name: "invalid gateway",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image:    "app:latest",
				Gateways: []ServiceGatewayRoute{{Hostname: "", ContainerPort: 8080}},
			},
			wantErr: domain.ErrRequired,
		},
		{
			name: "invalid env empty secretref",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image: "app:latest",
				Env: map[string]valueobject.SecretRef{
					"API_KEY": {},
				},
			},
			wantErr: domain.ErrEmptyValue,
		},
		{
			name: "valid minimal",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image: "app:latest",
			},
			wantErr: nil,
		},
		{
			name: "valid full",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image: "app:latest",
				Ports: []ServicePort{{Container: 8080, Host: 80, Protocol: "tcp"}},
				Env: map[string]valueobject.SecretRef{
					"API_KEY": *valueobject.NewSecretRefPlain("secret"),
				},
				Secrets:     []string{"db_pass"},
				Healthcheck: &ServiceHealthcheck{Path: "/health"},
				Resources:   ServiceResources{CPU: "1", Memory: "512M"},
				Volumes:     []ServiceVolume{{Source: "/host", Target: "/data"}},
				Gateways:    []ServiceGatewayRoute{{Hostname: "example.com", ContainerPort: 8080, HTTPS: true}},
				Internal:    true,
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

func TestBizService_GetServer(t *testing.T) {
	s := BizService{ServiceBase: ServiceBase{Server: "my-server"}}
	if got := s.GetServer(); got != "my-server" {
		t.Errorf("GetServer() = %v, want my-server", got)
	}
}
