package valueobject

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSecretRef_MarshalYAML_ValueReceiver 锁定 MarshalYAML 值接收者行为：
// 无论以值还是指针形式序列化，plain / secret 两种形态都必须输出期望形状。
// 回归风险点：uptime-kuma 等类型以值内嵌 SecretRef，若退化为指针接收者，
// 值字段将序列化为 {} 导致配置丢失。
func TestSecretRef_MarshalYAML_ValueReceiver(t *testing.T) {
	tests := []struct {
		name string
		ref  *SecretRef
		want string
	}{
		{"secret value", NewSecretRefSecret("kuma_db_password"), "secret: kuma_db_password\n"},
		{"plain value", NewSecretRefPlain("plain-token"), "plain-token\n"},
		{"plain priority over secret", NewSecretRef("p", "s"), "secret: s\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.ref)
			if err != nil {
				t.Fatalf("marshal pointer failed: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("pointer marshal = %q, want %q", string(data), tt.want)
			}

			value := *tt.ref
			data, err = yaml.Marshal(value)
			if err != nil {
				t.Fatalf("marshal value failed: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("value marshal = %q, want %q", string(data), tt.want)
			}
		})
	}
}

// TestSecretRef_MarshalYAML_EmbeddedValue 验证以值内嵌在结构体中的 SecretRef
// 仍能正确序列化（yaml.v3 对 map/struct 的值字段调用值类型 Marshaler）。
func TestSecretRef_MarshalYAML_EmbeddedValue(t *testing.T) {
	type holder struct {
		Name     string               `yaml:"name"`
		Password SecretRef            `yaml:"password"`
		Env      map[string]SecretRef `yaml:"env,omitempty"`
	}
	h := holder{
		Name:     "svc",
		Password: *NewSecretRefSecret("admin_pw"),
		Env:      map[string]SecretRef{"API_KEY": *NewSecretRefPlain("plain-key")},
	}

	data, err := yaml.Marshal(h)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(data)
	for _, c := range []string{"name: svc", "password:", "secret: admin_pw", "API_KEY: plain-key"} {
		if !strings.Contains(s, c) {
			t.Errorf("embedded marshal missing %q\n---\n%s", c, s)
		}
	}

	var back holder
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.Password.Secret() != "admin_pw" {
		t.Errorf("roundtrip secret = %q, want admin_pw", back.Password.Secret())
	}
	envKey := back.Env["API_KEY"]
	if envKey.Plain() != "plain-key" {
		t.Errorf("roundtrip env plain = %q, want plain-key", envKey.Plain())
	}
}

func TestSecretRef_LogValue(t *testing.T) {
	tests := []struct {
		name string
		ref  *SecretRef
	}{
		{"plain value", NewSecretRefPlain("my-password")},
		{"secret reference", NewSecretRefSecret("secret-name")},
		{"both values", NewSecretRef("plain-value-123", "secret-ref-456")},
		{"empty", NewSecretRef("", "")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			logger.Info("test", "secret", tt.ref)

			output := buf.String()

			if tt.ref.Plain() != "" && strings.Contains(output, tt.ref.Plain()) {
				t.Errorf("LogValue leaked plain value %q in output: %s", tt.ref.Plain(), output)
			}
			if tt.ref.Secret() != "" && strings.Contains(output, tt.ref.Secret()) {
				t.Errorf("LogValue leaked secret reference %q in output: %s", tt.ref.Secret(), output)
			}
			if !strings.Contains(output, "***") {
				t.Errorf("LogValue did not mask secret, output: %s", output)
			}
		})
	}
}
