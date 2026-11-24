package entity

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"gopkg.in/yaml.v3"
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
			name:    "valid without container_port",
			route:   ServiceGatewayRoute{Hostname: "example.com", Path: "/api", HTTP: true},
			wantErr: nil,
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

func TestServiceGatewayRoute_UnmarshalYAML_DefaultPort(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantPort int
	}{
		{
			name:     "container_port specified",
			yamlData: "hostname: example.com\ncontainer_port: 8080\n",
			wantPort: 8080,
		},
		{
			name:     "container_port not specified, uses default 80",
			yamlData: "hostname: example.com\n",
			wantPort: 80,
		},
		{
			name:     "container_port with other fields",
			yamlData: "hostname: example.com\npath: /api\nhttp: true\n",
			wantPort: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var route ServiceGatewayRoute
			err := yaml.Unmarshal([]byte(tt.yamlData), &route)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if route.ContainerPort != tt.wantPort {
				t.Errorf("ContainerPort = %v, want %v", route.ContainerPort, tt.wantPort)
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

func TestServiceGatewayRoute_AllowedIPs_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		want     []string
	}{
		{
			name:     "not set",
			yamlData: "hostname: example.com\nhttp: true\n",
			want:     nil,
		},
		{
			name:     "single cidr",
			yamlData: "hostname: example.com\nallowed_ips:\n  - 10.72.0.0/24\n",
			want:     []string{"10.72.0.0/24"},
		},
		{
			name:     "multiple entries",
			yamlData: "hostname: example.com\nallowed_ips:\n  - 10.72.0.0/24\n  - 127.0.0.1\n",
			want:     []string{"10.72.0.0/24", "127.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var route ServiceGatewayRoute
			if err := yaml.Unmarshal([]byte(tt.yamlData), &route); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if tt.want == nil {
				if len(route.AllowedIPs) != 0 {
					t.Errorf("AllowedIPs = %v, want empty", route.AllowedIPs)
				}
				return
			}
			if len(route.AllowedIPs) != len(tt.want) {
				t.Fatalf("AllowedIPs = %v, want %v", route.AllowedIPs, tt.want)
			}
			for i := range tt.want {
				if route.AllowedIPs[i] != tt.want[i] {
					t.Errorf("AllowedIPs[%d] = %q, want %q", i, route.AllowedIPs[i], tt.want[i])
				}
			}
		})
	}
}

func TestServiceGatewayRoute_RobotsDenyAll_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantNil  bool
		wantTrue bool
	}{
		{
			name:     "not set",
			yamlData: "hostname: example.com\nhttp: true\n",
			wantNil:  true,
		},
		{
			name:     "explicit true",
			yamlData: "hostname: example.com\nrobots_deny_all: true\n",
			wantTrue: true,
		},
		{
			name:     "explicit false",
			yamlData: "hostname: example.com\nrobots_deny_all: false\n",
			wantNil:  false,
			wantTrue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var route ServiceGatewayRoute
			if err := yaml.Unmarshal([]byte(tt.yamlData), &route); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if tt.wantNil {
				if route.RobotsDenyAll != nil {
					t.Errorf("RobotsDenyAll = %v, want nil", *route.RobotsDenyAll)
				}
				return
			}
			if route.RobotsDenyAll == nil {
				t.Fatalf("RobotsDenyAll is nil, want non-nil")
			}
			if *route.RobotsDenyAll != tt.wantTrue {
				t.Errorf("RobotsDenyAll = %v, want %v", *route.RobotsDenyAll, tt.wantTrue)
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
			name: "missing image and external_backends",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
			},
			wantErr: domain.ErrRequired,
		},
		{
			name: "both image and external_backends",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				Image:            "app:latest",
				ExternalBackends: []string{"https://example.com"},
			},
			wantErr: domain.ErrInvalidFormat,
		},
		{
			name: "valid with external_backends only",
			service: BizService{
				Name: "api",
				ServiceBase: ServiceBase{
					Server: "server-1",
				},
				ExternalBackends: []string{"https://example.com"},
			},
			wantErr: nil,
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

func TestServiceGatewayRoute_WAF_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name                  string
		yamlData              string
		wantWAF               *RouteWAFConfig
		wantNil               bool
		wantMode              string
		wantRequestBodyAccess *bool
	}{
		{
			name:     "no waf block",
			yamlData: "hostname: example.com\nhttp: true\n",
			wantNil:  true,
		},
		{
			name:     "waf with mode detect",
			yamlData: "hostname: example.com\nwaf:\n  mode: detect\n",
			wantNil:  false,
			wantMode: "detect",
		},
		{
			name:     "waf with mode block",
			yamlData: "hostname: example.com\nwaf:\n  mode: block\n",
			wantNil:  false,
			wantMode: "block",
		},
		{
			name:     "waf with mode disabled",
			yamlData: "hostname: example.com\nwaf:\n  mode: disabled\n",
			wantNil:  false,
			wantMode: "disabled",
		},
		{
			name:                  "waf with request_body_access false",
			yamlData:              "hostname: example.com\nwaf:\n  mode: detect\n  request_body_access: false\n",
			wantNil:               false,
			wantMode:              "detect",
			wantRequestBodyAccess: boolPtr(false),
		},
		{
			name:                  "waf with request_body_access true",
			yamlData:              "hostname: example.com\nwaf:\n  request_body_access: true\n",
			wantNil:               false,
			wantRequestBodyAccess: boolPtr(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var route ServiceGatewayRoute
			if err := yaml.Unmarshal([]byte(tt.yamlData), &route); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if tt.wantNil {
				if route.WAF != nil {
					t.Errorf("WAF = %v, want nil", route.WAF)
				}
				return
			}
			if route.WAF == nil {
				t.Fatalf("WAF is nil, want non-nil")
			}
			if route.WAF.Mode != tt.wantMode {
				t.Errorf("WAF.Mode = %q, want %q", route.WAF.Mode, tt.wantMode)
			}
			if tt.wantRequestBodyAccess != nil {
				if route.WAF.RequestBodyAccess == nil || *route.WAF.RequestBodyAccess != *tt.wantRequestBodyAccess {
					t.Errorf("WAF.RequestBodyAccess = %v, want %v", route.WAF.RequestBodyAccess, *tt.wantRequestBodyAccess)
				}
			}
		})
	}
}

func TestServiceGatewayRoute_BlacklistMode_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantMode string
		wantSet  bool
	}{
		{
			name:     "not set",
			yamlData: "hostname: example.com\nhttp: true\n",
			wantSet:  false,
		},
		{
			name:     "blacklist_mode block",
			yamlData: "hostname: example.com\nblacklist_mode: block\n",
			wantMode: "block",
			wantSet:  true,
		},
		{
			name:     "blacklist_mode detect",
			yamlData: "hostname: example.com\nblacklist_mode: detect\n",
			wantMode: "detect",
			wantSet:  true,
		},
		{
			name:     "blacklist_mode disabled",
			yamlData: "hostname: example.com\nblacklist_mode: disabled\n",
			wantMode: "disabled",
			wantSet:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var route ServiceGatewayRoute
			if err := yaml.Unmarshal([]byte(tt.yamlData), &route); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if !tt.wantSet {
				if route.BlacklistMode != "" {
					t.Errorf("BlacklistMode = %q, want empty", route.BlacklistMode)
				}
				return
			}
			if route.BlacklistMode != tt.wantMode {
				t.Errorf("BlacklistMode = %q, want %q", route.BlacklistMode, tt.wantMode)
			}
		})
	}
}

func TestServiceGatewayRoute_BlacklistMode_Validate(t *testing.T) {
	tests := []struct {
		name    string
		route   ServiceGatewayRoute
		wantErr error
	}{
		{
			name:    "empty inherits global",
			route:   ServiceGatewayRoute{Hostname: "example.com"},
			wantErr: nil,
		},
		{
			name:    "block",
			route:   ServiceGatewayRoute{Hostname: "example.com", BlacklistMode: "block"},
			wantErr: nil,
		},
		{
			name:    "detect",
			route:   ServiceGatewayRoute{Hostname: "example.com", BlacklistMode: "detect"},
			wantErr: nil,
		},
		{
			name:    "disabled",
			route:   ServiceGatewayRoute{Hostname: "example.com", BlacklistMode: "disabled"},
			wantErr: nil,
		},
		{
			name:    "invalid mode",
			route:   ServiceGatewayRoute{Hostname: "example.com", BlacklistMode: "invalid"},
			wantErr: fmt.Errorf("blacklist_mode must be 'block', 'detect' or 'disabled', got 'invalid'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.route.Validate()
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

func boolPtr(b bool) *bool { return &b }

// ===== 密码门 auth 配置（内部PRD） =====

func TestServiceGatewayRoute_Auth_UnmarshalYAML(t *testing.T) {
	yamlData := `
hostname: gated.example.com
http: true
https: true
auth:
  password:
    secret: gate_demo_password
  ttl: 12h
  public_paths:
    - /healthz
    - /metrics
`
	var route ServiceGatewayRoute
	if err := yaml.Unmarshal([]byte(yamlData), &route); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if route.Auth == nil {
		t.Fatalf("Auth is nil, want non-nil")
	}
	if route.Auth.Password == nil {
		t.Fatalf("Auth.Password is nil")
	}
	if route.Auth.Password.Secret() != "gate_demo_password" {
		t.Errorf("Password.Secret() = %q, want gate_demo_password", route.Auth.Password.Secret())
	}
	if route.Auth.TTL != "12h" {
		t.Errorf("TTL = %q, want 12h", route.Auth.TTL)
	}
	if len(route.Auth.PublicPaths) != 2 || route.Auth.PublicPaths[0] != "/healthz" {
		t.Errorf("PublicPaths = %v", route.Auth.PublicPaths)
	}
}

func TestServiceGatewayRoute_Auth_UnmarshalYAML_PlainPassword(t *testing.T) {
	yamlData := `
hostname: gated.example.com
http: true
auth:
  password: "plain-secret"
`
	var route ServiceGatewayRoute
	if err := yaml.Unmarshal([]byte(yamlData), &route); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if route.Auth == nil || route.Auth.Password == nil {
		t.Fatalf("Auth/Password not parsed")
	}
	if route.Auth.Password.Plain() != "plain-secret" {
		t.Errorf("Password.Plain() = %q, want plain-secret", route.Auth.Password.Plain())
	}
}

func TestServiceGatewayRoute_Auth_NotSet(t *testing.T) {
	var route ServiceGatewayRoute
	if err := yaml.Unmarshal([]byte("hostname: example.com\nhttp: true\n"), &route); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if route.Auth != nil {
		t.Errorf("Auth = %v, want nil when not set", route.Auth)
	}
}

func TestServiceGatewayRoute_Auth_Validate(t *testing.T) {
	tests := []struct {
		name    string
		route   ServiceGatewayRoute
		wantErr string
	}{
		{
			name: "valid secret ref",
			route: ServiceGatewayRoute{
				Hostname: "gated.example.com",
				Auth: &RouteAuthConfig{
					Password:    valueobject.NewSecretRefSecret("gate_password"),
					TTL:         "24h",
					PublicPaths: []string{"/healthz"},
				},
			},
		},
		{
			name: "valid plain no ttl no paths",
			route: ServiceGatewayRoute{
				Hostname: "gated.example.com",
				Auth:     &RouteAuthConfig{Password: valueobject.NewSecretRefPlain("pw")},
			},
		},
		{
			name: "missing password block",
			route: ServiceGatewayRoute{
				Hostname: "gated.example.com",
				Auth:     &RouteAuthConfig{},
			},
			wantErr: "auth.password is required",
		},
		{
			name: "empty password value",
			route: ServiceGatewayRoute{
				Hostname: "gated.example.com",
				Auth:     &RouteAuthConfig{Password: valueobject.NewSecretRefPlain("")},
			},
			wantErr: "auth.password",
		},
		{
			name: "invalid ttl",
			route: ServiceGatewayRoute{
				Hostname: "gated.example.com",
				Auth: &RouteAuthConfig{
					Password: valueobject.NewSecretRefPlain("pw"),
					TTL:      "24hours",
				},
			},
			wantErr: "not a valid duration",
		},
		{
			name: "public path not slash prefixed",
			route: ServiceGatewayRoute{
				Hostname: "gated.example.com",
				Auth: &RouteAuthConfig{
					Password:    valueobject.NewSecretRefPlain("pw"),
					PublicPaths: []string{"healthz"},
				},
			},
			wantErr: "must start with /",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.route.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// ===== 逐条目 http→https 301 + 规范域收敛 + HSTS per-host（内部总体方案 W3-2 / D8） =====

// strPtrOf 测试辅助：字符串字面量转 *string
func strPtrOf(s string) *string { return &s }

// 三字段 yaml 解析：未配置保持零值（nil/空），配置了按声明类型解析
func TestServiceGatewayRoute_RedirectFields_UnmarshalYAML(t *testing.T) {
	t.Run("not set stays zero", func(t *testing.T) {
		var route ServiceGatewayRoute
		if err := yaml.Unmarshal([]byte("hostname: example.com\nhttp: true\n"), &route); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if route.RedirectHTTPS != nil {
			t.Errorf("RedirectHTTPS = %v, want nil", *route.RedirectHTTPS)
		}
		if route.RedirectHost != "" {
			t.Errorf("RedirectHost = %q, want empty", route.RedirectHost)
		}
		if route.HSTS != nil {
			t.Errorf("HSTS = %v, want nil", *route.HSTS)
		}
	})

	t.Run("explicit values parse", func(t *testing.T) {
		var route ServiceGatewayRoute
		yamlData := "hostname: www.example.com\nredirect_https: true\nredirect_host: example.com\nhsts: \"off\"\n"
		if err := yaml.Unmarshal([]byte(yamlData), &route); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if route.RedirectHTTPS == nil || !*route.RedirectHTTPS {
			t.Errorf("RedirectHTTPS = %v, want *true", route.RedirectHTTPS)
		}
		if route.RedirectHost != "example.com" {
			t.Errorf("RedirectHost = %q, want example.com", route.RedirectHost)
		}
		if route.HSTS == nil || *route.HSTS != "off" {
			t.Errorf("HSTS = %v, want *\"off\"", route.HSTS)
		}
	})

	t.Run("explicit redirect_https false is distinguishable from unset", func(t *testing.T) {
		var route ServiceGatewayRoute
		if err := yaml.Unmarshal([]byte("hostname: example.com\nredirect_https: false\n"), &route); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if route.RedirectHTTPS == nil || *route.RedirectHTTPS {
			t.Errorf("RedirectHTTPS = %v, want *false (nil = 未配置，false = 显式关闭)", route.RedirectHTTPS)
		}
	})
}

// redirect_host 纯主机名校验（内部总体方案 W3-2 / D8）：禁止 URL（"://"）与路径（"/"），
// 合法形态为纯主机名（可含端口）；空 = 不启用不校验；hsts / redirect_https 无额外校验
func TestServiceGatewayRoute_RedirectHost_Validate(t *testing.T) {
	tests := []struct {
		name         string
		redirectHost string
		hsts         *string
		wantErr      string
	}{
		{"empty host = disabled, passes", "", nil, ""},
		{"bare hostname passes", "example.com", nil, ""},
		{"hostname with port passes", "example.com:8443", nil, ""},
		{"url with scheme fails", "https://example.com", nil, "redirect_host"},
		{"path-like value fails", "example.com/path", nil, "redirect_host"},
		{"hsts custom literal passes", "example.com", strPtrOf("max-age=60"), ""},
		{"hsts off passes", "example.com", strPtrOf("off"), ""},
		{"redirect_https needs no validation", "example.com", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := ServiceGatewayRoute{
				Hostname:     "www.example.com",
				HTTP:         true,
				RedirectHost: tt.redirectHost,
				HSTS:         tt.hsts,
			}
			err := route.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
			if !errors.Is(err, domain.ErrInvalidDomain) {
				t.Errorf("Validate() error = %v, want wrapping domain.ErrInvalidDomain", err)
			}
		})
	}
}

// ===== gate 安全加固新增字段（M3 防爆破参数 / L3 per-host WSS 跳过） =====

func TestRouteAuthConfig_M3Fields_Validate(t *testing.T) {
	pw := valueobject.NewSecretRefPlain("pw")

	tests := []struct {
		name    string
		auth    RouteAuthConfig
		wantErr string // 空 = 期望成功
	}{
		{"defaults ok", RouteAuthConfig{Password: pw}, ""},
		{"positive fields ok", RouteAuthConfig{Password: pw, MaxFailures: 5, LockDuration: "15m"}, ""},
		{"negative max_failures", RouteAuthConfig{Password: pw, MaxFailures: -1}, "max_failures"},
		{"invalid lock_duration", RouteAuthConfig{Password: pw, LockDuration: "abc"}, "lock_duration"},
		{"zero lock_duration", RouteAuthConfig{Password: pw, LockDuration: "0s"}, "lock_duration"},
		{"negative lock_duration", RouteAuthConfig{Password: pw, LockDuration: "-5m"}, "lock_duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Validate() error = nil, want containing %q", tt.wantErr)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestServiceGatewayRoute_WebSocketInsecureSkip_UnmarshalYAML(t *testing.T) {
	data := `
hostname: ws.env-dev.mesh.example.net
http: true
https: true
web_socket_insecure_skip: true
`
	var r ServiceGatewayRoute
	if err := yaml.Unmarshal([]byte(data), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.WebSocketInsecureSkip == nil || !*r.WebSocketInsecureSkip {
		t.Errorf("WebSocketInsecureSkip = %v, want true", r.WebSocketInsecureSkip)
	}

	var unset ServiceGatewayRoute
	if err := yaml.Unmarshal([]byte("hostname: h\nhttp: true\n"), &unset); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if unset.WebSocketInsecureSkip != nil {
		t.Errorf("unset WebSocketInsecureSkip must stay nil, got %v", unset.WebSocketInsecureSkip)
	}
}
