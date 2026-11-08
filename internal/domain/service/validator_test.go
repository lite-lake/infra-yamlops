package service

import (
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
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
				Name: "service1",
				ServiceBase: entity.ServiceBase{
					Server: "server1",
				},
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
				Name: "service1",
				ServiceBase: entity.ServiceBase{
					Server: "nonexistent",
				},
				Image: "nginx",
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
				Name: "service1",
				ServiceBase: entity.ServiceBase{
					Server: "server1",
				},
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
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}},
				{Name: "gw2", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "server2"}, Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("service port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{
				{Name: "svc1", ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", Ports: []entity.ServicePort{{Container: 80, Host: 8080}}},
				{Name: "svc2", ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", Ports: []entity.ServicePort{{Container: 80, Host: 8080}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected port conflict error, got %v", err)
		}
	})

	t.Run("gateway http port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}},
				{Name: "gw2", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 8443}},
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
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}},
				{Name: "gw2", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "server1"}, Image: "nginx", GatewayPorts: &entity.GatewayPorts{HTTP: 8080, HTTPS: 443}},
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
					Name: "svc1",
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Image: "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
				{
					Name: "svc2",
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Image: "nginx",
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
					Name: "svc1",
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Image: "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
				{
					Name: "svc2",
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Image: "nginx",
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

	t.Run("same hostname on different servers allowed", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{
				{Name: "server1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
				{Name: "server2", Zone: "zone1", SSH: entity.ServerSSH{Host: "5.6.7.8", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Services: []entity.BizService{
				{
					Name: "svc1",
					ServiceBase: entity.ServiceBase{
						Server: "server1",
					},
					Image: "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
				{
					Name: "svc2",
					ServiceBase: entity.ServiceBase{
						Server: "server2",
					},
					Image: "nginx",
					Gateways: []entity.ServiceGatewayRoute{
						{Hostname: "api.example.com", ContainerPort: 80, HTTP: true},
					},
				},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error for same hostname on different servers: %v", err)
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

func TestValidator_TunnelServerPortConflicts(t *testing.T) {
	t.Run("no conflict tunnel on different server", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{
				{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "gate", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}},
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv2"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("tunnel bind_port conflict with gateway port", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "gate", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, Ports: []entity.ServicePort{{Container: 30443, Host: 30070}}},
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected port conflict error, got %v", err)
		}
	})

	t.Run("two tunnel-servers same bind_port", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}}}},
				{Name: "frps2", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "b.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "bind_port") {
			t.Errorf("expected bind_port conflict error, got %v", err)
		}
	})

	t.Run("tunnel default bind_port conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "gate", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, Ports: []entity.ServicePort{{Container: 30070, Host: 30070}}},
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected port conflict error for default bind_port, got %v", err)
		}
	})
}

func TestValidator_TunnelHostnameConflicts(t *testing.T) {
	t.Run("no conflict tunnel and biz different hostnames", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{
				{Name: "svc1", ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "nginx", Gateways: []entity.ServiceGatewayRoute{{Hostname: "api.example.com", ContainerPort: 80, HTTP: true}}},
			},
			InfraServices: []entity.InfraService{
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "pay.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("tunnel hostname conflicts with biz hostname", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}},
			Services: []entity.BizService{
				{Name: "svc1", ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "nginx", Gateways: []entity.ServiceGatewayRoute{{Hostname: "pay.example.com", ContainerPort: 80, HTTP: true}}},
			},
			InfraServices: []entity.InfraService{
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "pay.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "hostname conflict") {
			t.Errorf("expected hostname conflict error, got %v", err)
		}
	})

	t.Run("duplicate tunnel route hostnames across servers", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "us-east-1"}},
			Servers: []entity.Server{
				{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			InfraServices: []entity.InfraService{
				{Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "pay.example.com"}}}},
				{Name: "frps2", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv2"}, Image: "frps", TunnelServerConfig: &entity.TunnelServerConfig{Ports: entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80}, AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "pay.example.com"}}}},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "hostname conflict") {
			t.Errorf("expected hostname conflict error, got %v", err)
		}
	})
}

func easyTierTestNode(name, server, vip string) entity.InfraService {
	return entity.InfraService{
		Name:        name,
		Type:        entity.InfraServiceTypeEasyTier,
		ServiceBase: entity.ServiceBase{Server: server},
		Image:       "easytier:v2.6.4",
		EasyTierConfig: &entity.EasyTierConfig{
			Role:          "shared",
			VirtualIP:     vip,
			NetworkName:   "lite-mesh",
			NetworkSecret: *valueobject.NewSecretRefSecret("mesh_secret"),
			Listeners:     []string{"quic://0.0.0.0:11010"},
		},
	}
}

func TestValidator_EasyTierPortConflicts(t *testing.T) {
	server := entity.Server{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}

	t.Run("two easytier nodes same listener port on same server", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{server},
			Secrets: []entity.Secret{{Name: "mesh_secret", Value: "s"}},
			InfraServices: []entity.InfraService{
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
				easyTierTestNode("et2", "srv1", "10.71.0.2/24"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "port conflict") {
			t.Errorf("expected easytier port conflict error, got %v", err)
		}
	})

	t.Run("easytier listener conflicts with gateway extra port", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{server},
			Secrets: []entity.Secret{{Name: "mesh_secret", Value: "s"}},
			InfraServices: []entity.InfraService{
				{Name: "gw1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "gate", GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443}, Ports: []entity.ServicePort{{Container: 30443, Host: 11010}}},
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "easytier listener port") {
			t.Errorf("expected easytier listener port conflict, got %v", err)
		}
	})

	t.Run("easytier nodes different servers no conflict", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{
				server,
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Secrets: []entity.Secret{{Name: "mesh_secret", Value: "s"}},
			InfraServices: []entity.InfraService{
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
				easyTierTestNode("et2", "srv2", "10.71.0.2/24"),
			},
		}
		validator := NewValidator(cfg)
		if err := validator.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidator_EasyTierMesh(t *testing.T) {
	server := entity.Server{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}

	t.Run("valid mesh", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{
				server,
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Secrets: []entity.Secret{{Name: "mesh_secret", Value: "s"}},
			InfraServices: []entity.InfraService{
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
				easyTierTestNode("et2", "srv2", "10.71.0.2/24"),
			},
		}
		validator := NewValidator(cfg)
		if err := validator.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate virtual_ip", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{
				server,
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Secrets: []entity.Secret{{Name: "mesh_secret", Value: "s"}},
			InfraServices: []entity.InfraService{
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
				easyTierTestNode("et2", "srv2", "10.71.0.1/24"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "virtual_ip") {
			t.Errorf("expected virtual_ip conflict, got %v", err)
		}
	})

	t.Run("inconsistent network_name", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{
				server,
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Secrets: []entity.Secret{{Name: "mesh_secret", Value: "s"}},
			InfraServices: []entity.InfraService{
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
				func() entity.InfraService {
					n := easyTierTestNode("et2", "srv2", "10.71.0.2/24")
					n.EasyTierConfig.NetworkName = "other-mesh"
					return n
				}(),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "network_name") {
			t.Errorf("expected network_name conflict, got %v", err)
		}
	})

	t.Run("inconsistent network_secret", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{
				server,
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Secrets: []entity.Secret{
				{Name: "mesh_secret", Value: "s"},
				{Name: "mesh_secret_other", Value: "t"},
			},
			InfraServices: []entity.InfraService{
				easyTierTestNode("et1", "srv1", "10.71.0.1/24"),
				func() entity.InfraService {
					n := easyTierTestNode("et2", "srv2", "10.71.0.2/24")
					n.EasyTierConfig.NetworkSecret = *valueobject.NewSecretRefSecret("mesh_secret_other")
					return n
				}(),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "network_secret") {
			t.Errorf("expected network_secret conflict, got %v", err)
		}
	})
}

func uptimeKumaValidatorInfra(name, server, hostname string) entity.InfraService {
	return entity.InfraService{
		Name:        name,
		Type:        entity.InfraServiceTypeUptimeKuma,
		ServiceBase: entity.ServiceBase{Server: server},
		Image:       "kuma:2",
		UptimeKumaConfig: &entity.UptimeKumaConfig{
			Admin: entity.UptimeKumaAdminConfig{
				Username: "admin",
				Password: *valueobject.NewSecretRefSecret("kuma_admin_password"),
			},
			DB: entity.UptimeKumaDBConfig{
				Type:     "mariadb",
				Hostname: "192.168.0.1",
				Port:     3306,
				Name:     "kuma",
				Username: "kuma",
				Password: *valueobject.NewSecretRefSecret("kuma_db_password"),
			},
			Gateways: []entity.UptimeKumaGatewayConfig{{Hostname: hostname, HTTPS: true}},
			Sync:     entity.UptimeKumaSyncConfig{Image: "sync:1", ReconcileInterval: "60s"},
		},
	}
}

func validSrv() entity.Server {
	return entity.Server{Name: "srv1", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.4", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}}
}

func TestValidator_UptimeKumaHostnameConflicts(t *testing.T) {
	t.Run("no conflict different hostnames", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{
				{Name: "kuma_admin_password", Value: "a"},
				{Name: "kuma_db_password", Value: "d"},
			},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up1.example.com"),
				uptimeKumaValidatorInfra("kuma2", "srv1", "up2.example.com"),
			},
		}
		validator := NewValidator(cfg)
		if err := validator.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("two uptime-kuma same hostname same server conflicts", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{
				{Name: "kuma_admin_password", Value: "a"},
				{Name: "kuma_db_password", Value: "d"},
			},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com"),
				uptimeKumaValidatorInfra("kuma2", "srv1", "up.example.com"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "hostname conflict") {
			t.Errorf("expected hostname conflict, got %v", err)
		}
	})

	t.Run("uptime-kuma conflicts with biz hostname on same server", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{
				{Name: "kuma_admin_password", Value: "a"},
				{Name: "kuma_db_password", Value: "d"},
			},
			Services: []entity.BizService{
				{
					Name: "web", ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "nginx",
					Gateways: []entity.ServiceGatewayRoute{{Hostname: "up.example.com", ContainerPort: 80, HTTPS: true}},
				},
			},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "hostname conflict") {
			t.Errorf("expected hostname conflict with biz service, got %v", err)
		}
	})

	t.Run("same uptime-kuma hostname on different servers allowed", func(t *testing.T) {
		cfg := &entity.Config{
			Zones: []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{
				validSrv(),
				{Name: "srv2", Zone: "zone1", SSH: entity.ServerSSH{Host: "1.2.3.5", Port: 22, User: "root", Password: *valueobject.NewSecretRefPlain("pass")}},
			},
			Secrets: []entity.Secret{
				{Name: "kuma_admin_password", Value: "a"},
				{Name: "kuma_db_password", Value: "d"},
			},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com"),
				uptimeKumaValidatorInfra("kuma2", "srv2", "up.example.com"),
			},
		}
		validator := NewValidator(cfg)
		if err := validator.Validate(); err != nil {
			t.Errorf("unexpected error for same hostname on different servers: %v", err)
		}
	})

	t.Run("uptime-kuma conflicts with tunnel hostname", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{
				{Name: "kuma_admin_password", Value: "a"},
				{Name: "kuma_db_password", Value: "d"},
			},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "pay.example.com"),
				{
					Name: "frps1", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "frps",
					TunnelServerConfig: &entity.TunnelServerConfig{AuthToken: *valueobject.NewSecretRefPlain("tok"), TunnelRoutes: []entity.TunnelRoute{{Hostname: "pay.example.com"}}},
				},
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "hostname conflict") {
			t.Errorf("expected hostname conflict with tunnel, got %v", err)
		}
	})
}

func TestValidator_UptimeKumaSecretReferences(t *testing.T) {
	t.Run("secrets exist valid", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{
				{Name: "kuma_admin_password", Value: "a"},
				{Name: "kuma_db_password", Value: "d"},
			},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com"),
			},
		}
		validator := NewValidator(cfg)
		if err := validator.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing admin secret", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{{Name: "kuma_db_password", Value: "d"}},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "secret 'kuma_admin_password' referenced by infra_service 'kuma1'") {
			t.Errorf("expected missing admin secret error, got %v", err)
		}
	})

	t.Run("missing db secret", func(t *testing.T) {
		cfg := &entity.Config{
			Zones:   []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers: []entity.Server{validSrv()},
			Secrets: []entity.Secret{{Name: "kuma_admin_password", Value: "a"}},
			InfraServices: []entity.InfraService{
				uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com"),
			},
		}
		validator := NewValidator(cfg)
		err := validator.Validate()
		if err == nil || !strings.Contains(err.Error(), "secret 'kuma_db_password' referenced by infra_service 'kuma1'") {
			t.Errorf("expected missing db secret error, got %v", err)
		}
	})

	t.Run("plain passwords skip secret check", func(t *testing.T) {
		infra := uptimeKumaValidatorInfra("kuma1", "srv1", "up.example.com")
		infra.UptimeKumaConfig.Admin.Password = *valueobject.NewSecretRefPlain("plain-admin")
		infra.UptimeKumaConfig.DB.Password = *valueobject.NewSecretRefPlain("plain-db")
		cfg := &entity.Config{
			Zones:         []entity.Zone{{Name: "zone1", Region: "cn"}},
			Servers:       []entity.Server{validSrv()},
			Secrets:       []entity.Secret{},
			InfraServices: []entity.InfraService{infra},
		}
		validator := NewValidator(cfg)
		if err := validator.Validate(); err != nil {
			t.Errorf("plain password config should validate, got %v", err)
		}
	})
}
