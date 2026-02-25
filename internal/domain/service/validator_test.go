package service

import (
	"strings"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestValidator_Validate_NilConfig(t *testing.T) {
	validator := NewValidator(nil)
	err := validator.Validate()
	if err != domain.ErrConfigNotLoaded {
		t.Errorf("expected ErrConfigNotLoaded, got %v", err)
	}
}

func TestValidator_Validate_EmptyConfig(t *testing.T) {
	cfg := &entity.Config{}
	validator := NewValidator(cfg)
	err := validator.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidator_ISPReferences(t *testing.T) {
	t.Run("valid secret reference", func(t *testing.T) {
		cfg := &entity.Config{
			Secrets: []entity.Secret{{Name: "api_key", Value: "secret123"}},
			ISPs: []entity.ISP{{
				Name:        "isp1",
				Services:    []entity.ISPService{"server"},
				Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefSecret("api_key")},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing secret reference", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs: []entity.ISP{{
				Name:        "isp1",
				Services:    []entity.ISPService{"server"},
				Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefSecret("nonexistent")},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "secret 'nonexistent' referenced by isp") {
			t.Errorf("expected missing secret error, got %v", err)
		}
	})

	t.Run("secret name same as isp name should fail", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs: []entity.ISP{{
				Name:        "my_secret",
				Services:    []entity.ISPService{"server"},
				Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefSecret("my_secret")},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "secret 'my_secret' referenced by isp") {
			t.Errorf("expected missing secret error (bug: was checking isps instead of secrets), got %v", err)
		}
	})
}

func TestValidator_ZoneReferences(t *testing.T) {
	t.Run("valid zone isp reference", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs:  []entity.ISP{{Name: "isp1", Services: []entity.ISPService{"server"}, Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefPlain("val")}}},
			Zones: []entity.Zone{{Name: "zone1", ISP: "isp1", Region: "us-east-1"}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing zone isp reference", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", ISP: "nonexistent", Region: "us-east-1"}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "isp 'nonexistent' referenced by zone") {
			t.Errorf("expected missing isp error, got %v", err)
		}
	})
}

func TestValidator_ServerReferences(t *testing.T) {
	t.Run("valid server references", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs:  []entity.ISP{{Name: "isp1", Services: []entity.ISPService{"server"}, Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefPlain("val")}}},
			Zones: []entity.Zone{{Name: "zone1", ISP: "isp1", Region: "us-east-1"}},
			Servers: []entity.Server{{
				Name: "server1",
				Zone: "zone1",
				ISP:  "isp1",
				SSH:  entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing zone reference", func(t *testing.T) {
		cfg := &entity.Config{
			Servers: []entity.Server{{
				Name: "server1",
				Zone: "nonexistent",
				SSH:  entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "zone 'nonexistent' referenced by server") {
			t.Errorf("expected missing zone error, got %v", err)
		}
	})
}

func TestValidator_ServiceReferences(t *testing.T) {
	t.Run("valid service references", func(t *testing.T) {
		cfg := &entity.Config{
			Secrets: []entity.Secret{{Name: "db_pass", Value: "secret123"}},
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{{
				Name:    "service1",
				Server:  "server1",
				Image:   "nginx",
				Secrets: []string{"db_pass"},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing server reference", func(t *testing.T) {
		cfg := &entity.Config{
			Services: []entity.BizService{{
				Name:   "service1",
				Server: "nonexistent",
				Image:  "nginx",
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "server 'nonexistent' referenced by service") {
			t.Errorf("expected missing server error, got %v", err)
		}
	})

	t.Run("missing secret reference", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{{
				Name:    "service1",
				Server:  "server1",
				Image:   "nginx",
				Secrets: []string{"nonexistent"},
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "secret 'nonexistent' referenced by service") {
			t.Errorf("expected missing secret error, got %v", err)
		}
	})
}

func TestValidator_DomainReferences(t *testing.T) {
	t.Run("valid domain references", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs: []entity.ISP{{Name: "dns_isp", Services: []entity.ISPService{"dns"}, Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefPlain("val")}}},
			Domains: []entity.Domain{{
				Name:   "example.com",
				DNSISP: "dns_isp",
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing dns_isp reference", func(t *testing.T) {
		cfg := &entity.Config{
			Domains: []entity.Domain{{
				Name:   "example.com",
				DNSISP: "nonexistent",
			}},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "dns_isp 'nonexistent' referenced by domain") {
			t.Errorf("expected missing dns_isp error, got %v", err)
		}
	})
}

func TestValidator_PortConflicts(t *testing.T) {
	t.Run("no conflict different servers", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{
				{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
				{Name: "server2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, Server: "server1", Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, GatewayConfig: &entity.GatewayConfig{Source: "test", Sync: true}},
				{Name: "gw2", Type: entity.InfraServiceTypeGateway, Server: "server2", Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, GatewayConfig: &entity.GatewayConfig{Source: "test", Sync: true}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("http port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{
				{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			InfraServices: []entity.InfraService{
				{Name: "ssl1", Type: entity.InfraServiceTypeSSL, Server: "server1", Image: "nginx", SSLConfig: &entity.SSLConfig{Ports: entity.SSLPorts{API: 80}, Config: &entity.SSLVolumeConfig{Source: "volumes://ssl", Sync: true}}},
				{Name: "ssl2", Type: entity.InfraServiceTypeSSL, Server: "server1", Image: "nginx", SSLConfig: &entity.SSLConfig{Ports: entity.SSLPorts{API: 80}, Config: &entity.SSLVolumeConfig{Source: "volumes://ssl", Sync: true}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected port conflict error, got %v", err)
		}
	})

	t.Run("service port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{
				{Name: "svc1", Server: "server1", Image: "nginx", Ports: []entity.ServicePort{{Container: 80, Host: 8080}}},
				{Name: "svc2", Server: "server1", Image: "nginx", Ports: []entity.ServicePort{{Container: 80, Host: 8080}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected port conflict error, got %v", err)
		}
	})

	t.Run("infra and service port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "ssl1", Type: entity.InfraServiceTypeSSL, Server: "server1", Image: "nginx", SSLConfig: &entity.SSLConfig{Ports: entity.SSLPorts{API: 8443}, Config: &entity.SSLVolumeConfig{Source: "volumes://ssl", Sync: true}}},
			},
			Services: []entity.BizService{
				{Name: "svc1", Server: "server1", Image: "nginx", Ports: []entity.ServicePort{{Container: 80, Host: 8443}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected port conflict error between infra and service, got %v", err)
		}
	})

	t.Run("gateway http port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, Server: "server1", Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, GatewayConfig: &entity.GatewayConfig{Source: "test", Sync: true}},
				{Name: "gw2", Type: entity.InfraServiceTypeGateway, Server: "server1", Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 8443}, GatewayConfig: &entity.GatewayConfig{Source: "test", Sync: true}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "gateway http port") {
			t.Errorf("expected gateway http port conflict error, got %v", err)
		}
	})

	t.Run("gateway https port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, Server: "server1", Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, GatewayConfig: &entity.GatewayConfig{Source: "test", Sync: true}},
				{Name: "gw2", Type: entity.InfraServiceTypeGateway, Server: "server1", Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 8080, HTTPS: 443}, GatewayConfig: &entity.GatewayConfig{Source: "test", Sync: true}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "gateway https port") {
			t.Errorf("expected gateway https port conflict error, got %v", err)
		}
	})
}

func TestValidator_HostnameConflicts(t *testing.T) {
	t.Run("no conflict different hostnames", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{
				{
					Name:   "svc1",
					Server: "server1",
					Image:  "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
				{
					Name:   "svc2",
					Server: "server1",
					Image:  "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "web.example.com", ContainerPort: 80, HTTP: true},
					},
				},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("hostname conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{
				{
					Name:   "svc1",
					Server: "server1",
					Image:  "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
				{
					Name:   "svc2",
					Server: "server1",
					Image:  "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "hostname conflict") {
			t.Errorf("expected hostname conflict error, got %v", err)
		}
	})
}

func TestValidator_DomainConflicts(t *testing.T) {
	t.Run("no conflict unique domains", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs: []entity.ISP{{Name: "dns_isp", Services: []entity.ISPService{"dns"}, Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefPlain("val")}}},
			Domains: []entity.Domain{
				{Name: "example.com", DNSISP: "dns_isp"},
				{Name: "example.org", DNSISP: "dns_isp"},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate domain", func(t *testing.T) {
		cfg := &entity.Config{
			ISPs: []entity.ISP{{Name: "dns_isp", Services: []entity.ISPService{"dns"}, Credentials: map[string]valueobject.SecretRef{"key": *valueobject.NewSecretRefPlain("val")}}},
			Domains: []entity.Domain{
				{Name: "example.com", DNSISP: "dns_isp"},
				{Name: "example.com", DNSISP: "dns_isp"},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "domain conflict") {
			t.Errorf("expected domain conflict error, got %v", err)
		}
	})
}
