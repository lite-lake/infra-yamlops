package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func TestNoopHandler_EntityType(t *testing.T) {
	h := NewNoopHandler("zone")
	if h.EntityType() != "zone" {
		t.Errorf("expected 'zone', got %s", h.EntityType())
	}
}

func TestNoopHandler_Apply(t *testing.T) {
	h := NewNoopHandler("test")
	ctx := context.Background()
	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "test", "test-entity")

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestServerHandler_EntityType(t *testing.T) {
	h := NewServerHandler()
	if h.EntityType() != "server" {
		t.Errorf("expected 'server', got %s", h.EntityType())
	}
}

func TestServerHandler_Apply_Create(t *testing.T) {
	h := NewServerHandler()
	ctx := context.Background()
	deps := newMockDeps()
	deps.SetServers(map[string]*ServerInfo{"server1": {Host: "1.2.3.4"}})

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "server", "server1").
		WithNewState(&entity.Server{Name: "server1", Zone: "zone1"})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v, warnings: %v", result.Error, result.Warnings)
	}
	// Output should contain "server registered" (without registries configured)
	if !strings.Contains(result.Output, "server registered") {
		t.Errorf("expected output to contain 'server registered', got: %s", result.Output)
	}
}

func TestServerHandler_Apply_Update(t *testing.T) {
	h := NewServerHandler()
	ctx := context.Background()
	deps := newMockDeps()
	deps.SetServers(map[string]*ServerInfo{"server1": {Host: "1.2.3.4"}})

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "server", "server1").
		WithNewState(&entity.Server{Name: "server1", Zone: "zone1"})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v, warnings: %v", result.Error, result.Warnings)
	}
	// Output should contain "server updated" (without registries configured)
	if !strings.Contains(result.Output, "server updated") {
		t.Errorf("expected output to contain 'server updated', got: %s", result.Output)
	}
}

func TestServerHandler_Apply_Delete(t *testing.T) {
	h := NewServerHandler()
	ctx := context.Background()
	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "server", "server1")

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Output != "server removed" {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestServerHandler_Apply_Noop(t *testing.T) {
	h := NewServerHandler()
	ctx := context.Background()
	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeNoop, "server", "server1")

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Output != "no action needed" {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestBaseDeps_New(t *testing.T) {
	d := NewBaseDeps()
	if d == nil {
		t.Fatal("expected non-nil BaseDeps")
	}
}

func TestBaseDeps_Options(t *testing.T) {
	secrets := map[string]string{"key": "value"}
	domains := map[string]*entity.Domain{"example.com": {Name: "example.com"}}
	isps := map[string]*entity.ISP{"cloudflare": {Name: "cloudflare"}}
	servers := map[string]*ServerInfo{"server1": {Host: "1.2.3.4"}}
	serverEntities := map[string]*entity.Server{"server1": {Name: "server1"}}
	registries := map[string]*entity.Registry{"reg1": {Name: "reg1"}}
	mockSSH := &mockSSHClient{}

	d := NewBaseDeps(
		WithSecrets(secrets),
		WithDomains(domains),
		WithISPs(isps),
		WithServers(servers),
		WithServerEntities(serverEntities),
		WithWorkDir("/opt/test"),
		WithEnv("production"),
		WithRegistries(registries),
		WithSSHClient(mockSSH, nil),
		WithDNSFactory(nil),
	)

	if d.Secrets()["key"] != "value" {
		t.Error("secrets not set correctly")
	}
	if got, ok := d.Domain("example.com"); !ok || got.Name != "example.com" {
		t.Error("domains not set correctly")
	}
	if isp, ok := d.ISP("cloudflare"); !ok || isp.Name != "cloudflare" {
		t.Error("ISPs not set correctly")
	}
	if info, ok := d.ServerInfo("server1"); !ok || info.Host != "1.2.3.4" {
		t.Error("servers not set correctly")
	}
	if d.WorkDir() != "/opt/test" {
		t.Errorf("expected /opt/test, got %s", d.WorkDir())
	}
	if d.Env() != "production" {
		t.Errorf("expected production, got %s", d.Env())
	}
	if d.RawSSHClient() != mockSSH {
		t.Error("SSH client not set correctly")
	}
}

func TestBaseDeps_NewWithNoOptions(t *testing.T) {
	d := NewBaseDeps()
	if d == nil {
		t.Fatal("expected non-nil BaseDeps")
	}
	if d.Secrets() == nil {
		t.Error("expected initialized secrets map")
	}
	if d.WorkDir() != "" {
		t.Error("expected empty workdir")
	}
}

func TestBaseDeps_SettersAndGetters(t *testing.T) {
	d := NewBaseDeps()

	d.SetWorkDir("/opt/test")
	if d.WorkDir() != "/opt/test" {
		t.Errorf("expected /opt/test, got %s", d.WorkDir())
	}

	d.SetEnv("production")
	if d.Env() != "production" {
		t.Errorf("expected production, got %s", d.Env())
	}

	secrets := map[string]string{"key": "value"}
	d.SetSecrets(secrets)
	if d.Secrets()["key"] != "value" {
		t.Error("secrets not set correctly")
	}

	domains := map[string]*entity.Domain{"example.com": {Name: "example.com"}}
	d.SetDomains(domains)
	got, ok := d.Domain("example.com")
	if !ok || got.Name != "example.com" {
		t.Error("domains not set correctly")
	}

	isps := map[string]*entity.ISP{"cloudflare": {Name: "cloudflare"}}
	d.SetISPs(isps)
	isp, ok := d.ISP("cloudflare")
	if !ok || isp.Name != "cloudflare" {
		t.Error("ISPs not set correctly")
	}

	servers := map[string]*ServerInfo{"server1": {Host: "1.2.3.4"}}
	d.SetServers(servers)
	info, ok := d.ServerInfo("server1")
	if !ok || info.Host != "1.2.3.4" {
		t.Error("servers not set correctly")
	}
}

func TestBaseDeps_SSHClient(t *testing.T) {
	t.Run("server not registered", func(t *testing.T) {
		d := NewBaseDeps()
		_, err := d.SSHClient("unknown")
		if !errors.Is(err, domainerr.ErrServerNotRegistered) {
			t.Errorf("expected ErrServerNotRegistered, got %v", err)
		}
	})

	t.Run("no ssh client", func(t *testing.T) {
		d := NewBaseDeps()
		d.SetServers(map[string]*ServerInfo{"server1": {}})
		_, err := d.SSHClient("server1")
		if !errors.Is(err, domainerr.ErrSSHClientNotAvailable) {
			t.Errorf("expected ErrSSHClientNotAvailable, got %v", err)
		}
	})

	t.Run("ssh error set", func(t *testing.T) {
		d := NewBaseDeps()
		d.SetServers(map[string]*ServerInfo{"server1": {}})
		customErr := errors.New("custom error")
		d.SetSSHClient(nil, customErr)
		_, err := d.SSHClient("server1")
		if !errors.Is(err, customErr) {
			t.Errorf("expected custom error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		d := NewBaseDeps()
		d.SetServers(map[string]*ServerInfo{"server1": {}})
		mockSSH := &mockSSHClient{}
		d.SetSSHClient(mockSSH, nil)
		client, err := d.SSHClient("server1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client != mockSSH {
			t.Error("expected mock SSH client")
		}
	})
}

func TestBaseDeps_Domain(t *testing.T) {
	d := NewBaseDeps()

	_, ok := d.Domain("nonexistent")
	if ok {
		t.Error("expected not found")
	}

	d.SetDomains(map[string]*entity.Domain{"example.com": {Name: "example.com"}})
	_, ok = d.Domain("example.com")
	if !ok {
		t.Error("expected found")
	}
}

func TestBaseDeps_ISP(t *testing.T) {
	d := NewBaseDeps()

	_, ok := d.ISP("nonexistent")
	if ok {
		t.Error("expected not found")
	}

	d.SetISPs(map[string]*entity.ISP{"cloudflare": {Name: "cloudflare"}})
	_, ok = d.ISP("cloudflare")
	if !ok {
		t.Error("expected found")
	}
}

func TestBaseDeps_ServerInfo(t *testing.T) {
	d := NewBaseDeps()

	_, ok := d.ServerInfo("nonexistent")
	if ok {
		t.Error("expected not found")
	}

	d.SetServers(map[string]*ServerInfo{"server1": {Host: "1.2.3.4"}})
	_, ok = d.ServerInfo("server1")
	if !ok {
		t.Error("expected found")
	}
}

func TestBaseDeps_ResolveSecret(t *testing.T) {
	d := NewBaseDeps()

	t.Run("plain text", func(t *testing.T) {
		val, err := d.ResolveSecret(valueobject.NewSecretRefPlain("secret123"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "secret123" {
			t.Errorf("expected secret123, got %s", val)
		}
	})

	t.Run("secret reference found", func(t *testing.T) {
		d.SetSecrets(map[string]string{"db_pass": "mypassword"})
		val, err := d.ResolveSecret(valueobject.NewSecretRefSecret("db_pass"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "mypassword" {
			t.Errorf("expected mypassword, got %s", val)
		}
	})

	t.Run("secret reference not found", func(t *testing.T) {
		_, err := d.ResolveSecret(valueobject.NewSecretRefSecret("nonexistent"))
		if err == nil {
			t.Error("expected error for missing secret")
		}
	})
}

func TestBaseDeps_DNSProvider(t *testing.T) {
	t.Run("ISP not found", func(t *testing.T) {
		d := NewBaseDeps()
		_, err := d.DNSProvider("nonexistent")
		if !errors.Is(err, domainerr.ErrISPNotFound) {
			t.Errorf("expected ErrISPNotFound, got %v", err)
		}
	})

	t.Run("ISP no DNS service", func(t *testing.T) {
		d := NewBaseDeps()
		d.SetISPs(map[string]*entity.ISP{
			"isp1": {Name: "isp1", Services: []entity.ISPService{"server"}},
		})
		_, err := d.DNSProvider("isp1")
		if !errors.Is(err, domainerr.ErrISPNoDNSService) {
			t.Errorf("expected ErrISPNoDNSService, got %v", err)
		}
	})
}

func TestBaseDeps_RawMethods(t *testing.T) {
	d := NewBaseDeps()

	mockSSH := &mockSSHClient{}
	d.SetSSHClient(mockSSH, nil)

	if d.RawSSHClient() != mockSSH {
		t.Error("RawSSHClient not returning correct client")
	}

	customErr := errors.New("test error")
	d.SetSSHClient(nil, customErr)
	if d.RawSSHError() != customErr {
		t.Error("RawSSHError not returning correct error")
	}
}
