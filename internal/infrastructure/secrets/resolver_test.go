package secrets

import (
	"testing"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestSecretResolver_Resolve(t *testing.T) {
	secrets := []*entity.Secret{
		{Name: "db-password", Value: "super-secret-123"},
		{Name: "api-key", Value: "key-abc-xyz"},
	}
	resolver := NewSecretResolver(secrets)

	t.Run("resolve secret reference", func(t *testing.T) {
		ref := valueobject.NewSecretRefSecret("db-password")
		val, err := resolver.Resolve(*ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "super-secret-123" {
			t.Errorf("expected 'super-secret-123', got %q", val)
		}
	})

	t.Run("resolve plain value", func(t *testing.T) {
		ref := valueobject.NewSecretRefPlain("plain-password")
		val, err := resolver.Resolve(*ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "plain-password" {
			t.Errorf("expected 'plain-password', got %q", val)
		}
	})

	t.Run("missing secret returns error", func(t *testing.T) {
		ref := valueobject.NewSecretRefSecret("non-existent")
		_, err := resolver.Resolve(*ref)
		if err == nil {
			t.Error("expected error for missing secret")
		}
	})
}

func TestSecretResolver_GetResolvedValue(t *testing.T) {
	secrets := []*entity.Secret{
		{Name: "cached-secret", Value: "cached-value"},
	}
	resolver := NewSecretResolver(secrets)

	ref := valueobject.NewSecretRefSecret("cached-secret")
	resolver.cacheResolved(*ref, "cached-value")

	val := resolver.GetResolvedValue(*ref)
	if val != "cached-value" {
		t.Errorf("expected 'cached-value', got %q", val)
	}
}

func TestSecretResolver_ResolveAll_DoesNotModifyOriginal(t *testing.T) {
	secrets := []*entity.Secret{
		{Name: "test-secret", Value: "secret-value"},
	}
	resolver := NewSecretResolver(secrets)

	cfg := &entity.Config{
		Servers: []entity.Server{
			{
				Name: "test-server",
				SSH: entity.ServerSSH{
					Password: *valueobject.NewSecretRefSecret("test-secret"),
				},
			},
		},
	}

	originalSecretRef := cfg.Servers[0].SSH.Password

	err := resolver.ResolveAll(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Servers[0].SSH.Password.Plain() != "" {
		t.Errorf("ResolveAll modified Password.Plain, expected empty, got %q", cfg.Servers[0].SSH.Password.Plain())
	}

	if cfg.Servers[0].SSH.Password.Secret() != originalSecretRef.Secret() {
		t.Errorf("ResolveAll modified Password.Secret, expected %q, got %q", originalSecretRef.Secret(), cfg.Servers[0].SSH.Password.Secret())
	}
}

func TestSecretResolver_ResolveAll_CachesValues(t *testing.T) {
	secrets := []*entity.Secret{
		{Name: "server-password", Value: "resolved-password"},
	}
	resolver := NewSecretResolver(secrets)

	cfg := &entity.Config{
		Servers: []entity.Server{
			{
				Name: "test-server",
				SSH: entity.ServerSSH{
					Password: *valueobject.NewSecretRefSecret("server-password"),
				},
			},
		},
	}

	err := resolver.ResolveAll(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ref := cfg.Servers[0].SSH.Password
	val := resolver.GetResolvedValue(ref)
	if val != "resolved-password" {
		t.Errorf("GetResolvedValue returned %q, expected 'resolved-password'", val)
	}
}

func TestSecretResolver_ResolveAll_ISPCredentials(t *testing.T) {
	secrets := []*entity.Secret{
		{Name: "api-token", Value: "token-123"},
	}
	resolver := NewSecretResolver(secrets)

	cfg := &entity.Config{
		ISPs: []entity.ISP{
			{
				Name: "cloudflare",
				Credentials: map[string]valueobject.SecretRef{
					"api_token": *valueobject.NewSecretRefSecret("api-token"),
				},
			},
		},
	}

	err := resolver.ResolveAll(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ref := cfg.ISPs[0].Credentials["api_token"]
	if ref.Plain() != "" {
		t.Errorf("ResolveAll modified ISP credential Plain field")
	}

	resolvedVal := resolver.GetResolvedValue(ref)
	if resolvedVal != "token-123" {
		t.Errorf("GetResolvedValue returned %q, expected 'token-123'", resolvedVal)
	}
}

func TestSecretResolver_ResolveAll_RegistryCredentials(t *testing.T) {
	secrets := []*entity.Secret{
		{Name: "registry-user", Value: "admin"},
		{Name: "registry-pass", Value: "password123"},
	}
	resolver := NewSecretResolver(secrets)

	cfg := &entity.Config{
		Registries: []entity.Registry{
			{
				Name: "docker-hub",
				Credentials: entity.RegistryCredentials{
					Username: *valueobject.NewSecretRefSecret("registry-user"),
					Password: *valueobject.NewSecretRefSecret("registry-pass"),
				},
			},
		},
	}

	err := resolver.ResolveAll(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userRef := cfg.Registries[0].Credentials.Username
	passRef := cfg.Registries[0].Credentials.Password
	if userRef.Plain() != "" {
		t.Errorf("ResolveAll modified Registry username Plain field")
	}
	if passRef.Plain() != "" {
		t.Errorf("ResolveAll modified Registry password Plain field")
	}

	username := resolver.GetResolvedValue(userRef)
	password := resolver.GetResolvedValue(passRef)

	if username != "admin" {
		t.Errorf("GetResolvedValue for username returned %q, expected 'admin'", username)
	}
	if password != "password123" {
		t.Errorf("GetResolvedValue for password returned %q, expected 'password123'", password)
	}
}
