package gate

// 产物级回归：ssl.allow_insecure_http 必须真实落盘——曾因生成器
// 字段静默丢失（entity → GatewayConfig 已接线但 sslConfig.Remote 未接线）
// 导致远端产物缺键、网关 F6.1 启动校验崩溃循环（部署成功 ≠ 配置正确）。

import (
	"strings"
	"testing"
)

func sslTestConfig(allow *bool) *GatewayConfig {
	return &GatewayConfig{
		Port:                 80,
		SSLMode:              "remote",
		SSLEndpoint:          "http://ssl-api-prod/cert/json",
		SSLAPIKey:            "test-key",
		SSLAllowInsecureHTTP: allow,
	}
}

// TestGenerateSSLAllowInsecureHTTPExplicitTrue 显式 true：产物必须包含该键
func TestGenerateSSLAllowInsecureHTTPExplicitTrue(t *testing.T) {
	b := true
	out, err := NewGenerator().Generate(sslTestConfig(&b), nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(out, "allow_insecure_http: true") {
		t.Errorf("artifact missing allow_insecure_http: true (silent field drop):\n%s", out)
	}
}

// TestGenerateSSLAllowInsecureHTTPNil nil：产物不生成该键（存量输出逐字节不变）
func TestGenerateSSLAllowInsecureHTTPNil(t *testing.T) {
	out, err := NewGenerator().Generate(sslTestConfig(nil), nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if strings.Contains(out, "allow_insecure_http") {
		t.Errorf("artifact must not emit allow_insecure_http when nil:\n%s", out)
	}
	if !strings.Contains(out, "endpoint:") || !strings.Contains(out, "http://ssl-api-prod/cert/json") {
		t.Errorf("artifact missing ssl remote endpoint:\n%s", out)
	}
}
