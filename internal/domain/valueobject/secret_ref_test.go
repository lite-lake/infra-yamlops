package valueobject

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretRef_LogValue(t *testing.T) {
	tests := []struct {
		name string
		ref  *SecretRef
	}{
		{"plain value", NewSecretRefPlain("my-password")},
		{"secret reference", NewSecretRefSecret("secret-name")},
		{"both values", NewSecretRef("plain-value-123", "secret-ref-456")},
		{"empty", &SecretRef{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			logger.Info("test", "secret", tt.ref)

			output := buf.String()

			if tt.ref.Plain != "" && strings.Contains(output, tt.ref.Plain) {
				t.Errorf("LogValue leaked plain value %q in output: %s", tt.ref.Plain, output)
			}
			if tt.ref.Secret != "" && strings.Contains(output, tt.ref.Secret) {
				t.Errorf("LogValue leaked secret reference %q in output: %s", tt.ref.Secret, output)
			}
			if !strings.Contains(output, "***") {
				t.Errorf("LogValue did not mask secret, output: %s", output)
			}
		})
	}
}
