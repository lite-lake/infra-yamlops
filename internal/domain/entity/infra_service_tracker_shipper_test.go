package entity

import (
	"errors"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"gopkg.in/yaml.v3"
)

// ===== tracker_shipper 校验（内部PRD §7.1） =====

func TestGatewayTrackerShipperConfig_Validate(t *testing.T) {
	valid := func(c *GatewayTrackerShipperConfig) *GatewayTrackerShipperConfig {
		if c.SharedSecret.Plain() == "" && c.SharedSecret.Secret() == "" {
			c.SharedSecret = *valueobject.NewSecretRefPlain("s3cret")
		}
		return c
	}

	tests := []struct {
		name    string
		config  *GatewayTrackerShipperConfig
		wantErr error
	}{
		{
			name:   "disabled minimal valid",
			config: &GatewayTrackerShipperConfig{Enabled: false},
		},
		{
			name:   "enabled complete valid",
			config: valid(&GatewayTrackerShipperConfig{Enabled: true, Endpoint: "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest", NodeName: "yo-dev-gateway-node1"}),
		},
		{
			name:    "enabled missing endpoint",
			config:  valid(&GatewayTrackerShipperConfig{Enabled: true, NodeName: "yo-dev-gateway-node1"}),
			wantErr: domain.ErrRequired,
		},
		{
			name:    "enabled missing shared_secret",
			config:  &GatewayTrackerShipperConfig{Enabled: true, Endpoint: "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest", NodeName: "yo-dev-gateway-node1"},
			wantErr: domain.ErrEmptyValue,
		},
		{
			// node_name 缺省合法：生成器自动注入 yo-{env}-{服务名}，产物保证非空（生成层兜底）
			name:   "enabled missing node_name ok (generator auto-injects)",
			config: valid(&GatewayTrackerShipperConfig{Enabled: true, Endpoint: "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest"}),
		},
		{
			name:    "invalid flush_interval",
			config:  &GatewayTrackerShipperConfig{FlushInterval: "abc"},
			wantErr: domain.ErrInvalidDuration,
		},
		{
			name:    "non positive flush_interval",
			config:  &GatewayTrackerShipperConfig{FlushInterval: "0s"},
			wantErr: domain.ErrInvalidDuration,
		},
		{
			name:    "negative buffer_size",
			config:  &GatewayTrackerShipperConfig{BufferSize: -1},
			wantErr: domain.ErrInvalidFormat,
		},
		{
			name:    "negative batch_size",
			config:  &GatewayTrackerShipperConfig{BatchSize: -5},
			wantErr: domain.ErrInvalidFormat,
		},
		{
			name:    "failure_sample_log_ratio above 1",
			config:  &GatewayTrackerShipperConfig{FailureSampleLogRatio: 1.5},
			wantErr: domain.ErrInvalidFormat,
		},
		{
			name:    "negative failure_sample_log_ratio",
			config:  &GatewayTrackerShipperConfig{FailureSampleLogRatio: -0.01},
			wantErr: domain.ErrInvalidFormat,
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

// ===== InfraService tracker_shipper YAML 解析/序列化 =====

// TestInfraService_TrackerShipper_Unmarshal 验证 gateway 类型服务解析 tracker_shipper 段：
// secret 引用与 plain 明文两种形态，子字段全部就位。
func TestInfraService_TrackerShipper_Unmarshal(t *testing.T) {
	raw := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
ports:
  http: 80
  https: 443
tracker_shipper:
  enabled: true
  endpoint: "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest"
  shared_secret:
    secret: tc_gateway_ingest_secret
  node_name: yo-dev-gateway-node1
  buffer_size: 20000
  batch_size: 800
  flush_interval: 2s
  failure_sample_log_ratio: 0.05
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(raw), &svc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	ts := svc.GatewayTrackerShipper
	if ts == nil {
		t.Fatalf("GatewayTrackerShipper is nil, want parsed config")
	}
	if !ts.Enabled {
		t.Error("Enabled = false, want true")
	}
	if ts.Endpoint != "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest" {
		t.Errorf("Endpoint = %q", ts.Endpoint)
	}
	if ts.SharedSecret.Secret() != "tc_gateway_ingest_secret" {
		t.Errorf("SharedSecret.Secret() = %q, want tc_gateway_ingest_secret", ts.SharedSecret.Secret())
	}
	if ts.NodeName != "yo-dev-gateway-node1" {
		t.Errorf("NodeName = %q, want yo-dev-gateway-node1", ts.NodeName)
	}
	if ts.BufferSize != 20000 {
		t.Errorf("BufferSize = %d, want 20000", ts.BufferSize)
	}
	if ts.BatchSize != 800 {
		t.Errorf("BatchSize = %d, want 800", ts.BatchSize)
	}
	if ts.FlushInterval != "2s" {
		t.Errorf("FlushInterval = %q, want 2s", ts.FlushInterval)
	}
	if ts.FailureSampleLogRatio != 0.05 {
		t.Errorf("FailureSampleLogRatio = %v, want 0.05", ts.FailureSampleLogRatio)
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

// TestInfraService_TrackerShipper_EnabledRequiresFields 验证实体层严格校验：
// enabled=true 时缺 endpoint/shared_secret 任一项均报错。
// node_name 不在必填之列：未配置时由生成器自动注入 yo-{env}-{服务名}
// （显式配置值优先，见 TestInfraService_TrackerShipper_EnabledNodeNameOptional）。
func TestInfraService_TrackerShipper_EnabledRequiresFields(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{
			name: "enabled missing endpoint",
			raw: `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
tracker_shipper:
  enabled: true
  shared_secret:
    secret: tc_gateway_ingest_secret
  node_name: yo-dev-gateway-node1
`,
			wantErr: domain.ErrRequired,
		},
		{
			name: "enabled missing shared_secret",
			raw: `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
tracker_shipper:
  enabled: true
  endpoint: "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest"
  node_name: yo-dev-gateway-node1
`,
			wantErr: domain.ErrEmptyValue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc InfraService
			if err := yaml.Unmarshal([]byte(tt.raw), &svc); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			err := svc.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestInfraService_TrackerShipper_EnabledNodeNameOptional 验证 enabled=true 且未配置
// node_name 时实体校验通过：node_name 可省略，生成器自动注入 yo-{env}-{服务名}
// （LoadAndValidate 先于生成执行，实体层不得把自动注入堵成死路径）。
func TestInfraService_TrackerShipper_EnabledNodeNameOptional(t *testing.T) {
	raw := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
tracker_shipper:
  enabled: true
  endpoint: "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest"
  shared_secret:
    secret: tc_gateway_ingest_secret
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(raw), &svc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if svc.GatewayTrackerShipper == nil {
		t.Fatalf("GatewayTrackerShipper is nil, want parsed config")
	}
	if svc.GatewayTrackerShipper.NodeName != "" {
		t.Errorf("NodeName = %q, want empty (unset)", svc.GatewayTrackerShipper.NodeName)
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil (node_name optional, generator auto-injects)", err)
	}
}

// TestInfraService_TrackerShipper_PlainSecretUnmarshal 验证 shared_secret plain 明文形态。
func TestInfraService_TrackerShipper_PlainSecretUnmarshal(t *testing.T) {
	raw := `
name: gateway-node1
type: gateway
server: srv-node1
image: gate:latest
tracker_shipper:
  enabled: false
  shared_secret: plain-secret-value
`
	var svc InfraService
	if err := yaml.Unmarshal([]byte(raw), &svc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	ts := svc.GatewayTrackerShipper
	if ts == nil {
		t.Fatalf("GatewayTrackerShipper is nil, want parsed config")
	}
	if ts.SharedSecret.Plain() != "plain-secret-value" {
		t.Errorf("SharedSecret.Plain() = %q, want plain-secret-value", ts.SharedSecret.Plain())
	}
}

// TestInfraService_TrackerShipper_MarshalRoundTrip 验证序列化保留 tracker_shipper，
// 且未配置的服务不输出该键。
func TestInfraService_TrackerShipper_MarshalRoundTrip(t *testing.T) {
	svc := InfraService{
		Name:  "gateway-node1",
		Type:  InfraServiceTypeGateway,
		Image: "gate:latest",
		ServiceBase: ServiceBase{
			Server: "srv-node1",
		},
		GatewayTrackerShipper: &GatewayTrackerShipperConfig{
			Enabled:      true,
			Endpoint:     "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
			SharedSecret: *valueobject.NewSecretRefSecret("tc_gateway_ingest_secret"),
			NodeName:     "yo-dev-gateway-node1",
		},
	}
	out, err := yaml.Marshal(&svc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{
		"tracker_shipper:",
		"enabled: true",
		"endpoint: https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
		"secret: tc_gateway_ingest_secret",
		"node_name: yo-dev-gateway-node1",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("marshaled output missing %q, got:\n%s", want, out)
		}
	}

	// 未配置 tracker_shipper：marshal 输出不含该键
	plain := InfraService{
		Name:  "gateway-node2",
		Type:  InfraServiceTypeGateway,
		Image: "gate:latest",
		ServiceBase: ServiceBase{
			Server: "srv-node2",
		},
	}
	plainOut, err := yaml.Marshal(&plain)
	if err != nil {
		t.Fatalf("Marshal(plain): %v", err)
	}
	if strings.Contains(string(plainOut), "tracker_shipper") {
		t.Errorf("marshal output must not contain tracker_shipper when unset, got:\n%s", plainOut)
	}
}
