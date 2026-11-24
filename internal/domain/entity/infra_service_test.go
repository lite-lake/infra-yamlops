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
		{
			// 双钥匙：明文 http 端点必须显式放行，否则 fail-fast
			name:    "plaintext http endpoint without allow rejected",
			config:  GatewaySSLConfig{Mode: "remote", Endpoint: "http://ssl-api-prod/cert/json"},
			wantErr: fmt.Errorf(`ssl.endpoint "http://ssl-api-prod/cert/json" uses plaintext http; set ssl.allow_insecure_http: true to allow（F6.1 内网明文逃生口；对齐 gate 启动校验，避免远端崩溃循环）`),
		},
		{
			name:    "plaintext http endpoint with allow passes",
			config:  GatewaySSLConfig{Mode: "remote", Endpoint: "http://ssl-api-prod/cert/json", AllowInsecureHTTP: sslAllowPtr(true)},
			wantErr: nil,
		},
		{
			name:    "https endpoint needs no allow",
			config:  GatewaySSLConfig{Mode: "remote", Endpoint: "https://ssl.example.com/cert/json"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				// 混合比较：sentinel 用 errors.Is；fmt.Errorf 动态文案用全文比对
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
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
		{
			name:    "valid empty crs_failure_policy",
			config:  GatewayWAFConfig{Enabled: true, CRSFailurePolicy: ""},
			wantErr: nil,
		},
		{
			name:    "valid crs_failure_policy degrade with acknowledged",
			config:  GatewayWAFConfig{Enabled: true, CRSFailurePolicy: "degrade", CRSDegradeAcknowledged: true},
			wantErr: nil,
		},
		{
			// 双钥匙：degrade 必须显式承认，否则 fail-fast
			name:    "degrade without acknowledged rejected",
			config:  GatewayWAFConfig{Enabled: true, CRSFailurePolicy: "degrade"},
			wantErr: fmt.Errorf("waf.crs_failure_policy=degrade 需同时显式 waf.crs_degrade_acknowledged: true（承认无 WAF 降级运行；对齐 gate F11 双钥匙启动校验）"),
		},
		{
			name:    "valid crs_failure_policy fail",
			config:  GatewayWAFConfig{Enabled: true, CRSFailurePolicy: "fail"},
			wantErr: nil,
		},
		{
			name:    "invalid crs_failure_policy",
			config:  GatewayWAFConfig{Enabled: true, CRSFailurePolicy: "retry"},
			wantErr: fmt.Errorf("waf.crs_failure_policy must be 'degrade' or 'fail', got 'retry'"),
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

func TestGatewayBlacklistConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  GatewayBlacklistConfig
		wantErr error
	}{
		{
			name:    "valid minimal",
			config:  GatewayBlacklistConfig{Enabled: true},
			wantErr: nil,
		},
		{
			name:    "valid default_mode block",
			config:  GatewayBlacklistConfig{Enabled: true, DefaultMode: "block"},
			wantErr: nil,
		},
		{
			name:    "valid default_mode detect",
			config:  GatewayBlacklistConfig{Enabled: true, DefaultMode: "detect"},
			wantErr: nil,
		},
		{
			name:    "valid default_mode disabled",
			config:  GatewayBlacklistConfig{Enabled: true, DefaultMode: "disabled"},
			wantErr: nil,
		},
		{
			name:    "invalid default_mode",
			config:  GatewayBlacklistConfig{Enabled: true, DefaultMode: "invalid"},
			wantErr: fmt.Errorf("blacklist.default_mode must be 'block', 'detect' or 'disabled', got 'invalid'"),
		},
		{
			name:    "valid whitelist single IP",
			config:  GatewayBlacklistConfig{Enabled: true, Whitelist: []string{"203.0.113.10"}},
			wantErr: nil,
		},
		{
			name:    "valid whitelist CIDR",
			config:  GatewayBlacklistConfig{Enabled: true, Whitelist: []string{"10.0.0.0/8"}},
			wantErr: nil,
		},
		{
			name:    "valid whitelist mixed IP and CIDR",
			config:  GatewayBlacklistConfig{Enabled: true, Whitelist: []string{"203.0.113.10", "10.0.0.0/8", "2001:db8::1"}},
			wantErr: nil,
		},
		{
			name:    "invalid whitelist entry",
			config:  GatewayBlacklistConfig{Enabled: true, Whitelist: []string{"not-an-ip"}},
			wantErr: domain.ErrInvalidIP,
		},
		{
			name:    "valid source_url https",
			config:  GatewayBlacklistConfig{Enabled: true, SourceURL: "https://blackip.ustc.edu.cn/list.php?txt"},
			wantErr: nil,
		},
		{
			name:    "valid source_url http",
			config:  GatewayBlacklistConfig{Enabled: true, SourceURL: "http://mirror.example.com/list.txt"},
			wantErr: nil,
		},
		{
			name:    "invalid source_url scheme",
			config:  GatewayBlacklistConfig{Enabled: true, SourceURL: "ftp://example.com/list.txt"},
			wantErr: domain.ErrInvalidURL,
		},
		{
			name:    "invalid source_url missing host",
			config:  GatewayBlacklistConfig{Enabled: true, SourceURL: "https://"},
			wantErr: domain.ErrInvalidURL,
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

func TestInfraService_UnmarshalYAML_GatewayBlacklist(t *testing.T) {
	yamlStr := `
name: gateway-node5
type: gateway
server: srv-node5
image: gate:latest
blacklist:
  enabled: true
  default_mode: detect
  source_url: https://mirror.example.com/list.txt
  whitelist:
    - 203.0.113.10
    - 10.0.0.0/8
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.GatewayBlacklist == nil {
		t.Fatal("GatewayBlacklist is nil, want non-nil")
	}
	if !svc.GatewayBlacklist.Enabled {
		t.Error("GatewayBlacklist.Enabled = false, want true")
	}
	if svc.GatewayBlacklist.DefaultMode != "detect" {
		t.Errorf("GatewayBlacklist.DefaultMode = %q, want detect", svc.GatewayBlacklist.DefaultMode)
	}
	if svc.GatewayBlacklist.SourceURL != "https://mirror.example.com/list.txt" {
		t.Errorf("GatewayBlacklist.SourceURL = %q", svc.GatewayBlacklist.SourceURL)
	}
	wantWhitelist := []string{"203.0.113.10", "10.0.0.0/8"}
	if len(svc.GatewayBlacklist.Whitelist) != len(wantWhitelist) {
		t.Fatalf("GatewayBlacklist.Whitelist = %v, want %v", svc.GatewayBlacklist.Whitelist, wantWhitelist)
	}
	for i := range wantWhitelist {
		if svc.GatewayBlacklist.Whitelist[i] != wantWhitelist[i] {
			t.Errorf("GatewayBlacklist.Whitelist[%d] = %q, want %q", i, svc.GatewayBlacklist.Whitelist[i], wantWhitelist[i])
		}
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestInfraService_UnmarshalYAML_GatewayBlacklistNotSet(t *testing.T) {
	yamlStr := `
name: gateway-node5
type: gateway
server: srv-node5
image: gate:latest
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.GatewayBlacklist != nil {
		t.Errorf("GatewayBlacklist = %v, want nil", svc.GatewayBlacklist)
	}
}

// TestInfraService_UnmarshalYAML_GatewayWAF_CRSFailurePolicy 验证 services_infra.yaml 的
// waf.crs_failure_policy 解析（内部PRD §5.7），未配置时保持零值。
func TestInfraService_UnmarshalYAML_GatewayWAF_CRSFailurePolicy(t *testing.T) {
	yamlStr := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
waf:
  enabled: false
  default_mode: detect
  crs_mirror_url: "https://crsmirror.env-dev.mesh.example.net"
  crs_failure_policy: degrade
  crs_degrade_acknowledged: true
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.GatewayWAF == nil {
		t.Fatal("GatewayWAF is nil, want non-nil")
	}
	if svc.GatewayWAF.CRSFailurePolicy != "degrade" {
		t.Errorf("GatewayWAF.CRSFailurePolicy = %q, want degrade", svc.GatewayWAF.CRSFailurePolicy)
	}
	if svc.GatewayWAF.CRSMirrorURL != "https://crsmirror.env-dev.mesh.example.net" {
		t.Errorf("GatewayWAF.CRSMirrorURL = %q", svc.GatewayWAF.CRSMirrorURL)
	}
	if !svc.GatewayWAF.CRSDegradeAcknowledged {
		t.Errorf("GatewayWAF.CRSDegradeAcknowledged = false, want true")
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}

	// 未配置：零值（不下发，gate 侧默认 fail）
	var legacy InfraService
	legacyStr := `
name: gateway-node5
type: gateway
server: srv-node5
image: gate:latest
waf:
  enabled: false
`
	if err := yaml.Unmarshal([]byte(legacyStr), &legacy); err != nil {
		t.Fatalf("UnmarshalYAML(legacy) failed: %v", err)
	}
	if legacy.GatewayWAF == nil {
		t.Fatal("GatewayWAF is nil, want non-nil")
	}
	if legacy.GatewayWAF.CRSFailurePolicy != "" {
		t.Errorf("legacy GatewayWAF.CRSFailurePolicy = %q, want empty", legacy.GatewayWAF.CRSFailurePolicy)
	}
}

func TestInfraService_Validate_GatewayBlacklist(t *testing.T) {
	svc := InfraService{
		Name: "gateway-node5",
		Type: InfraServiceTypeGateway,
		ServiceBase: ServiceBase{
			Server: "srv-node5",
		},
		Image: "gate:latest",
		GatewayBlacklist: &GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "bogus",
		},
	}
	if err := svc.Validate(); err == nil {
		t.Error("Validate() expected error for invalid blacklist default_mode, got nil")
	}
}

func TestValidateGatewayTrustedProxies(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		wantErr error
	}{
		{name: "nil entries", entries: nil, wantErr: nil},
		{name: "empty entries", entries: []string{}, wantErr: nil},
		{name: "valid ipv4 CIDR", entries: []string{"10.72.0.0/24"}, wantErr: nil},
		{name: "valid ipv6 CIDR", entries: []string{"2001:db8::/64"}, wantErr: nil},
		{name: "valid bare IP", entries: []string{"10.72.0.5"}, wantErr: nil},
		{name: "valid mixed CIDR and IP", entries: []string{"10.72.0.0/24", "10.71.0.9", "fd00::5"}, wantErr: nil},
		{
			name:    "invalid format",
			entries: []string{"10.72.0.0/99"},
			wantErr: domain.ErrInvalidCIDR,
		},
		{
			name:    "not a cidr nor ip",
			entries: []string{"mesh-network"},
			wantErr: domain.ErrInvalidCIDR,
		},
		{
			name:    "empty entry",
			entries: []string{""},
			wantErr: domain.ErrInvalidCIDR,
		},
		{
			name:    "catch-all ipv4 forbidden",
			entries: []string{"10.72.0.0/24", "0.0.0.0/0"},
			wantErr: domain.ErrInvalidCIDR,
		},
		{
			name:    "catch-all ipv6 forbidden",
			entries: []string{"::/0"},
			wantErr: domain.ErrInvalidCIDR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGatewayTrustedProxies(tt.entries)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("validateGatewayTrustedProxies(%v) expected error, got nil", tt.entries)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("validateGatewayTrustedProxies(%v) error = %v, want %v", tt.entries, err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("validateGatewayTrustedProxies(%v) unexpected error = %v", tt.entries, err)
			}
		})
	}
}

func TestInfraService_UnmarshalYAML_GatewayTrustedProxies(t *testing.T) {
	yamlStr := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
trusted_proxies:
  - 10.72.0.0/24
  - 10.71.0.7
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	want := []string{"10.72.0.0/24", "10.71.0.7"}
	if len(svc.GatewayTrustedProxies) != len(want) {
		t.Fatalf("GatewayTrustedProxies = %v, want %v", svc.GatewayTrustedProxies, want)
	}
	for i := range want {
		if svc.GatewayTrustedProxies[i] != want[i] {
			t.Errorf("GatewayTrustedProxies[%d] = %q, want %q", i, svc.GatewayTrustedProxies[i], want[i])
		}
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestInfraService_UnmarshalYAML_GatewayTrustedProxiesNotSet(t *testing.T) {
	yamlStr := `
name: gateway-node5
type: gateway
server: srv-node5
image: gate:latest
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.GatewayTrustedProxies != nil {
		t.Errorf("GatewayTrustedProxies = %v, want nil", svc.GatewayTrustedProxies)
	}
}

func TestInfraService_MarshalYAML_GatewayTrustedProxies(t *testing.T) {
	t.Run("configured emits trusted_proxies", func(t *testing.T) {
		svc := InfraService{
			Name: "gateway-node1",
			Type: InfraServiceTypeGateway,
			ServiceBase: ServiceBase{
				Server: "srv-node1",
			},
			Image:                 "gate:latest",
			GatewayTrustedProxies: []string{"10.72.0.0/24"},
		}
		data, err := yaml.Marshal(&svc)
		if err != nil {
			t.Fatalf("MarshalYAML failed: %v", err)
		}
		var parsed struct {
			TrustedProxies []string `yaml:"trusted_proxies"`
		}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("re-unmarshal failed: %v\n%s", err, data)
		}
		if len(parsed.TrustedProxies) != 1 || parsed.TrustedProxies[0] != "10.72.0.0/24" {
			t.Errorf("trusted_proxies = %v, want [10.72.0.0/24], got:\n%s", parsed.TrustedProxies, data)
		}
	})

	t.Run("not configured omits trusted_proxies", func(t *testing.T) {
		svc := InfraService{
			Name: "gateway-node5",
			Type: InfraServiceTypeGateway,
			ServiceBase: ServiceBase{
				Server: "srv-node5",
			},
			Image: "gate:latest",
		}
		data, err := yaml.Marshal(&svc)
		if err != nil {
			t.Fatalf("MarshalYAML failed: %v", err)
		}
		if strings.Contains(string(data), "trusted_proxies") {
			t.Errorf("marshal output must not contain trusted_proxies when unset, got:\n%s", data)
		}
	})
}

func TestInfraService_Validate_GatewayTrustedProxies(t *testing.T) {
	t.Run("invalid entry rejected", func(t *testing.T) {
		svc := InfraService{
			Name: "gateway-node1",
			Type: InfraServiceTypeGateway,
			ServiceBase: ServiceBase{
				Server: "srv-node1",
			},
			Image:                 "gate:latest",
			GatewayTrustedProxies: []string{"10.72.0.0/24", "not-a-cidr"},
		}
		err := svc.Validate()
		if err == nil {
			t.Fatal("Validate() expected error for invalid trusted_proxies entry, got nil")
		}
		if !errors.Is(err, domain.ErrInvalidCIDR) {
			t.Errorf("Validate() error = %v, want wrapping %v", err, domain.ErrInvalidCIDR)
		}
	})

	t.Run("catch-all rejected", func(t *testing.T) {
		svc := InfraService{
			Name: "gateway-node1",
			Type: InfraServiceTypeGateway,
			ServiceBase: ServiceBase{
				Server: "srv-node1",
			},
			Image:                 "gate:latest",
			GatewayTrustedProxies: []string{"0.0.0.0/0"},
		}
		if err := svc.Validate(); !errors.Is(err, domain.ErrInvalidCIDR) {
			t.Errorf("Validate() error = %v, want wrapping %v", err, domain.ErrInvalidCIDR)
		}
	})

	t.Run("mesh segment accepted", func(t *testing.T) {
		svc := InfraService{
			Name: "gateway-node1",
			Type: InfraServiceTypeGateway,
			ServiceBase: ServiceBase{
				Server: "srv-node1",
			},
			Image:                 "gate:latest",
			GatewayTrustedProxies: []string{"10.72.0.0/24"},
		}
		if err := svc.Validate(); err != nil {
			t.Errorf("Validate() unexpected error = %v", err)
		}
	})
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

func TestInfraService_UnmarshalYAML_EasyTier(t *testing.T) {
	yamlStr := `
name: easytier-node5
type: easytier
server: srv-node5
image: mirror.example.com/ghcr/easytier/easytier:v2.6.4
role: shared
virtual_ip: 10.72.0.11/24
network_name: lite-mesh-prod
network_secret: {secret: et_network_secret_prod}
hostname: srv-node5
listeners:
  - quic://0.0.0.0:11010
peers:
  - quic://103.236.57.87:11010
flags:
  latency_first: true
  private_mode: true
  relay_network_whitelist: ""
  relay_all_peer_rpc: true
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.Type != InfraServiceTypeEasyTier {
		t.Fatalf("Type = %q, want %q", svc.Type, InfraServiceTypeEasyTier)
	}
	if svc.EasyTierConfig == nil {
		t.Fatal("EasyTierConfig is nil")
	}
	et := svc.EasyTierConfig
	if et.Role != "shared" || et.VirtualIP != "10.72.0.11/24" || et.NetworkName != "lite-mesh-prod" {
		t.Errorf("EasyTierConfig = %+v, unexpected", et)
	}
	if et.NetworkSecret.Secret() != "et_network_secret_prod" {
		t.Errorf("NetworkSecret secret = %q, want et_network_secret_prod", et.NetworkSecret.Secret())
	}
	if len(et.Listeners) != 1 || et.Listeners[0] != "quic://0.0.0.0:11010" {
		t.Errorf("Listeners = %v, want [quic://0.0.0.0:11010]", et.Listeners)
	}
	if !et.Flags.PrivateMode || !et.Flags.LatencyFirst || !et.Flags.RelayAllPeerRPC {
		t.Errorf("Flags = %+v, unexpected", et.Flags)
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestInfraService_UnmarshalYAML_EasyTier_Leaf(t *testing.T) {
	yamlStr := `
name: easytier-node1
type: easytier
server: srv-node1
image: mirror.example.com/ghcr/easytier/easytier:v2.6.4
role: leaf
virtual_ip: 10.71.0.191/24
network_name: lite-mesh-dev
network_secret: {secret: et_network_secret_dev}
peers:
  - quic://154.37.215.114:11010
flags:
  latency_first: true
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.EasyTierConfig == nil {
		t.Fatal("EasyTierConfig is nil")
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestEasyTierConfig_Validate(t *testing.T) {
	validShared := func() *EasyTierConfig {
		return &EasyTierConfig{
			Role:          "shared",
			VirtualIP:     "10.72.0.11/24",
			NetworkName:   "lite-mesh-prod",
			NetworkSecret: *valueobject.NewSecretRefPlain("s"),
			Listeners:     []string{"quic://0.0.0.0:11010"},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*EasyTierConfig)
		wantErr error
	}{
		{"valid shared", func(c *EasyTierConfig) {}, nil},
		{"invalid role", func(c *EasyTierConfig) { c.Role = "hub" }, domain.ErrInvalidType},
		{"invalid ip non-cidr", func(c *EasyTierConfig) { c.VirtualIP = "10.0.0.1" }, domain.ErrInvalidCIDR},
		{"invalid ip bad addr", func(c *EasyTierConfig) { c.VirtualIP = "999.1.1.1/24" }, domain.ErrInvalidCIDR},
		{"missing network_name", func(c *EasyTierConfig) { c.NetworkName = "" }, domain.ErrRequired},
		{"missing secret", func(c *EasyTierConfig) { c.NetworkSecret = valueobject.SecretRef{} }, domain.ErrEmptyValue},
		{"shared no listeners", func(c *EasyTierConfig) { c.Listeners = nil }, domain.ErrRequired},
		{"shared invalid listener", func(c *EasyTierConfig) { c.Listeners = []string{"ftp://0.0.0.0:21"} }, domain.ErrInvalidType},
		{"shared with no_listener", func(c *EasyTierConfig) { c.Flags.NoListener = true }, domain.ErrInvalidType},
		{"leaf with listeners", func(c *EasyTierConfig) {
			c.Role = "leaf"
			c.Listeners = []string{"quic://0.0.0.0:11010"}
		}, domain.ErrInvalidType},
		{"leaf valid", func(c *EasyTierConfig) {
			c.Role = "leaf"
			c.Listeners = nil
		}, nil},
		{"invalid peer", func(c *EasyTierConfig) { c.Peers = []string{"not-a-uri"} }, domain.ErrInvalidType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validShared()
			tt.mutate(c)
			err := c.Validate()
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

func TestInfraService_UnmarshalYAML_GatewayResources(t *testing.T) {
	yamlStr := `
name: gateway-node6
type: gateway
server: srv-node6
image: gate:latest
resources:
  memory: "1G"
ports:
  http: 80
  https: 443
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.Resources.Memory != "1G" {
		t.Errorf("Resources.Memory = %q, want 1G", svc.Resources.Memory)
	}
	if svc.Resources.CPU != "" {
		t.Errorf("Resources.CPU = %q, want empty", svc.Resources.CPU)
	}
}

func TestInfraService_UnmarshalYAML_GatewayResourcesNotSet(t *testing.T) {
	yamlStr := `
name: gateway-node6
type: gateway
server: srv-node6
image: gate:latest
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(yamlStr), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.Resources != (ServiceResources{}) {
		t.Errorf("Resources = %v, want zero value", svc.Resources)
	}
}

// ===== gate 安全加固新增字段（H2 pin / 明文镜像放行 / L6 / H1 / L3） =====

func TestGatewayWAFConfig_Validate_CRSPinnedSHA256(t *testing.T) {
	tests := []struct {
		name    string
		pin     string
		wantErr bool
	}{
		{"empty ok (未配置)", "", false},
		{"64 hex ok", strings.Repeat("a", 64), false},
		{"sha256: prefix ok", "sha256:" + strings.Repeat("ab", 32), false},
		{"surrounding whitespace ok", "  " + strings.Repeat("cd", 32) + "  ", false},
		{"uppercase hex ok", strings.ToUpper(strings.Repeat("ab", 32)), false},
		{"too short", strings.Repeat("a", 63), true},
		{"too long", strings.Repeat("a", 65), true},
		{"non hex", strings.Repeat("g", 64), true},
		{"prefix only", "sha256:", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &GatewayWAFConfig{CRSPinnedSHA256: tt.pin}
			err := w.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "crs_pinned_sha256") {
				t.Errorf("error = %v, want mentioning crs_pinned_sha256", err)
			}
		})
	}
}

func TestGatewayWAFConfig_UnmarshalYAML_CRSPinAndInsecureHTTP(t *testing.T) {
	var s InfraService
	data := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
waf:
  enabled: true
  crs_pinned_sha256: "sha256:d50377c6eb7ead1da83ad428ba941f6c768234fb139679cb0a26663a767326f4"
  crs_mirror_allow_insecure_http: true
`
	if err := yaml.Unmarshal([]byte(data), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GatewayWAF == nil {
		t.Fatal("GatewayWAF = nil")
	}
	if s.GatewayWAF.CRSPinnedSHA256 != "sha256:d50377c6eb7ead1da83ad428ba941f6c768234fb139679cb0a26663a767326f4" {
		t.Errorf("CRSPinnedSHA256 = %q", s.GatewayWAF.CRSPinnedSHA256)
	}
	if s.GatewayWAF.CRSMirrorAllowInsecureHTTP == nil || !*s.GatewayWAF.CRSMirrorAllowInsecureHTTP {
		t.Errorf("CRSMirrorAllowInsecureHTTP = %v, want true", s.GatewayWAF.CRSMirrorAllowInsecureHTTP)
	}
	if err := s.GatewayWAF.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestGatewayBlacklistConfig_Validate_MaxEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries int
		wantErr bool
	}{
		{"zero ok (未配置)", 0, false},
		{"positive ok", 300000, false},
		{"negative rejected", -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &GatewayBlacklistConfig{MaxEntries: tt.entries}
			err := b.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestInfraService_UnmarshalYAML_GatewayServerSecurityFields(t *testing.T) {
	data := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
strict_path_check: false
websocket_insecure_skip: true
`
	var withFields InfraService
	if err := yaml.Unmarshal([]byte(data), &withFields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if withFields.GatewayStrictPathCheck == nil || *withFields.GatewayStrictPathCheck {
		t.Errorf("GatewayStrictPathCheck = %v, want false", withFields.GatewayStrictPathCheck)
	}
	if withFields.GatewayWebSocketInsecureSkip == nil || !*withFields.GatewayWebSocketInsecureSkip {
		t.Errorf("GatewayWebSocketInsecureSkip = %v, want true", withFields.GatewayWebSocketInsecureSkip)
	}

	var withoutFields InfraService
	if err := yaml.Unmarshal([]byte("name: g\ntype: gateway\nserver: s\nimage: i\n"), &withoutFields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if withoutFields.GatewayStrictPathCheck != nil || withoutFields.GatewayWebSocketInsecureSkip != nil {
		t.Errorf("unset fields must stay nil, got (%v, %v)",
			withoutFields.GatewayStrictPathCheck, withoutFields.GatewayWebSocketInsecureSkip)
	}
}

func TestInfraService_UnmarshalYAML_GatewayBlacklistL6Fields(t *testing.T) {
	data := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
blacklist:
  enabled: true
  allow_http_source: true
  max_entries: 300000
`
	var s InfraService
	if err := yaml.Unmarshal([]byte(data), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GatewayBlacklist == nil {
		t.Fatal("GatewayBlacklist = nil")
	}
	if s.GatewayBlacklist.AllowHTTPSource == nil || !*s.GatewayBlacklist.AllowHTTPSource {
		t.Errorf("AllowHTTPSource = %v, want true", s.GatewayBlacklist.AllowHTTPSource)
	}
	if s.GatewayBlacklist.MaxEntries != 300000 {
		t.Errorf("MaxEntries = %d, want 300000", s.GatewayBlacklist.MaxEntries)
	}
	if err := s.GatewayBlacklist.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func sslAllowPtr(b bool) *bool { return &b }
