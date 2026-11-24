package generator

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// TestCollectHostRoutes_ReadTimeoutPassthrough entity.read_timeout → gate.HostRoute
func TestCollectHostRoutes_ReadTimeoutPassthrough(t *testing.T) {
	route := entity.ServiceGatewayRoute{
		Hostname:    "sc.env-dev.mesh.example.net",
		HTTP:        true,
		HTTPS:       true,
		ReadTimeout: "30m",
	}
	gen, gw, config := authGatewayFixture(t, route)
	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts = %d, want 1", len(hosts))
	}
	if hosts[0].ReadTimeout != "30m" {
		t.Errorf("ReadTimeout = %q, want 30m", hosts[0].ReadTimeout)
	}

	// 未配置保持空串（产物不生成该键，gate 继承 120s）
	plain := entity.ServiceGatewayRoute{Hostname: "plain.env-dev.mesh.example.net", HTTP: true}
	gen2, gw2, cfg2 := authGatewayFixture(t, plain)
	hosts2, err := gen2.collectHostRoutes(gw2, cfg2)
	if err != nil {
		t.Fatalf("collectHostRoutes(plain): %v", err)
	}
	if hosts2[0].ReadTimeout != "" {
		t.Errorf("plain ReadTimeout = %q, want empty", hosts2[0].ReadTimeout)
	}
}

// TestGenerateGateYAML_ReadTimeout 生成产物 hosts[].read_timeout 逐键断言
func TestGenerateGateYAML_ReadTimeout(t *testing.T) {
	route := entity.ServiceGatewayRoute{
		Hostname:    "sc.env-dev.mesh.example.net",
		HTTP:        true,
		HTTPS:       true,
		ReadTimeout: "30m",
	}
	gen, gw, config := authGatewayFixture(t, route)
	result, err := gen.buildGatewayRoutes(gw, config)
	if err != nil {
		t.Fatalf("buildGatewayRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, content)
	}
	hosts, _ := doc["hosts"].([]interface{})
	var host map[string]interface{}
	for _, raw := range hosts {
		if m, ok := raw.(map[string]interface{}); ok && m["name"] == route.Hostname {
			host = m
			break
		}
	}
	if host == nil {
		t.Fatalf("host missing:\n%s", content)
	}
	if v, _ := host["read_timeout"].(string); v != "30m" {
		t.Errorf("hosts[].read_timeout = %v, want 30m\n%s", host["read_timeout"], content)
	}

	// 未配置时不得出现该键（存量输出兼容）
	plain := entity.ServiceGatewayRoute{Hostname: "plain.env-dev.mesh.example.net", HTTP: true}
	gen2, gw2, cfg2 := authGatewayFixture(t, plain)
	result2, err := gen2.buildGatewayRoutes(gw2, cfg2)
	if err != nil {
		t.Fatalf("buildGatewayRoutes(plain): %v", err)
	}
	content2, err := gen2.gateGen.Generate(result2.gatewayConfig, result2.hosts)
	if err != nil {
		t.Fatalf("Generate(plain): %v", err)
	}
	if strings.Contains(content2, "read_timeout") {
		t.Errorf("未配置 read_timeout 的产物不得出现该键:\n%s", content2)
	}
}

// TestServiceGatewayRoute_ValidateReadTimeout 非法 duration fail-fast
func TestServiceGatewayRoute_ValidateReadTimeout(t *testing.T) {
	cases := []struct {
		value string
		ok    bool
	}{
		{"", true},
		{"30m", true},
		{"1h30m", true},
		{"abc", false},
		{"0s", false},
		{"-5m", false},
	}
	for _, tc := range cases {
		r := entity.ServiceGatewayRoute{Hostname: "sc.example.com", ReadTimeout: tc.value}
		err := r.Validate()
		if tc.ok && err != nil {
			t.Errorf("read_timeout=%q 应通过校验, 实际: %v", tc.value, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("read_timeout=%q 应校验失败", tc.value)
		}
	}
}
