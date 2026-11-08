package service

import (
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func TestInfraServiceEquals_GatewayResources(t *testing.T) {
	base := &entity.InfraService{
		Name:      "gateway-node6",
		Type:      entity.InfraServiceTypeGateway,
		Image:     "gate:prod",
		Resources: entity.ServiceResources{Memory: "1G"},
	}

	t.Run("equal", func(t *testing.T) {
		b := &entity.InfraService{
			Name:      "gateway-node6",
			Type:      entity.InfraServiceTypeGateway,
			Image:     "gate:prod",
			Resources: entity.ServiceResources{Memory: "1G"},
		}
		if !InfraServiceEquals(base, b) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("different memory", func(t *testing.T) {
		b := &entity.InfraService{
			Name:      "gateway-node6",
			Type:      entity.InfraServiceTypeGateway,
			Image:     "gate:prod",
			Resources: entity.ServiceResources{Memory: "2G"},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different memory), got equal")
		}
	})

	t.Run("resources unset vs set", func(t *testing.T) {
		b := &entity.InfraService{
			Name:  "gateway-node6",
			Type:  entity.InfraServiceTypeGateway,
			Image: "gate:prod",
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (resources unset), got equal")
		}
	})
}

func TestInfraServiceEquals_TunnelServer(t *testing.T) {
	token := *valueobject.NewSecretRefPlain("secret-token")
	base := &entity.InfraService{
		Name:        "frps-node5",
		Type:        entity.InfraServiceTypeTunnelServer,
		Image:       "frps:prod",
		ServiceBase: entity.ServiceBase{Server: "srv-node5"},
		TunnelServerConfig: &entity.TunnelServerConfig{
			Ports:        entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80},
			AuthToken:    token,
			TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}, {Hostname: "b.example.com"}},
		},
	}

	t.Run("equal", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frps-node5",
			Type:        entity.InfraServiceTypeTunnelServer,
			Image:       "frps:prod",
			ServiceBase: entity.ServiceBase{Server: "srv-node5"},
			TunnelServerConfig: &entity.TunnelServerConfig{
				Ports:        entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80},
				AuthToken:    token,
				TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}, {Hostname: "b.example.com"}},
			},
		}
		if !InfraServiceEquals(base, b) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("different bind_port", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frps-node5",
			Type:        entity.InfraServiceTypeTunnelServer,
			Image:       "frps:prod",
			ServiceBase: entity.ServiceBase{Server: "srv-node5"},
			TunnelServerConfig: &entity.TunnelServerConfig{
				Ports:        entity.TunnelServerPorts{BindPort: 30071, VhostHTTPPort: 80},
				AuthToken:    token,
				TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}, {Hostname: "b.example.com"}},
			},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different bind_port), got equal")
		}
	})

	t.Run("different tunnel_routes", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frps-node5",
			Type:        entity.InfraServiceTypeTunnelServer,
			Image:       "frps:prod",
			ServiceBase: entity.ServiceBase{Server: "srv-node5"},
			TunnelServerConfig: &entity.TunnelServerConfig{
				Ports:        entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80},
				AuthToken:    token,
				TunnelRoutes: []entity.TunnelRoute{{Hostname: "a.example.com"}, {Hostname: "c.example.com"}},
			},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different tunnel_routes), got equal")
		}
	})

	t.Run("nil vs non-nil config", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frps-node5",
			Type:        entity.InfraServiceTypeTunnelServer,
			Image:       "frps:prod",
			ServiceBase: entity.ServiceBase{Server: "srv-node5"},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (nil config), got equal")
		}
	})
}

func TestInfraServiceEquals_TunnelClient(t *testing.T) {
	token := *valueobject.NewSecretRefPlain("secret-token")
	base := &entity.InfraService{
		Name:        "frpc-node1",
		Type:        entity.InfraServiceTypeTunnelClient,
		Image:       "frpc:dev",
		ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		TunnelClientConfig: &entity.TunnelClientConfig{
			ServerAddr: "frps.example.com:30070",
			AuthToken:  token,
			Proxies: []entity.TunnelClientProxy{
				{Name: "p1", Type: "http", LocalAddr: "gw:80", CustomDomains: []string{"a.com"}},
			},
		},
	}

	t.Run("equal", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frpc-node1",
			Type:        entity.InfraServiceTypeTunnelClient,
			Image:       "frpc:dev",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
			TunnelClientConfig: &entity.TunnelClientConfig{
				ServerAddr: "frps.example.com:30070",
				AuthToken:  token,
				Proxies: []entity.TunnelClientProxy{
					{Name: "p1", Type: "http", LocalAddr: "gw:80", CustomDomains: []string{"a.com"}},
				},
			},
		}
		if !InfraServiceEquals(base, b) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("different server_addr", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frpc-node1",
			Type:        entity.InfraServiceTypeTunnelClient,
			Image:       "frpc:dev",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
			TunnelClientConfig: &entity.TunnelClientConfig{
				ServerAddr: "frps2.example.com:30070",
				AuthToken:  token,
				Proxies: []entity.TunnelClientProxy{
					{Name: "p1", Type: "http", LocalAddr: "gw:80", CustomDomains: []string{"a.com"}},
				},
			},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different server_addr), got equal")
		}
	})

	t.Run("different proxy", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "frpc-node1",
			Type:        entity.InfraServiceTypeTunnelClient,
			Image:       "frpc:dev",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
			TunnelClientConfig: &entity.TunnelClientConfig{
				ServerAddr: "frps.example.com:30070",
				AuthToken:  token,
				Proxies: []entity.TunnelClientProxy{
					{Name: "p1", Type: "tcp", LocalAddr: "gw:3306", RemotePort: 3306},
				},
			},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different proxy), got equal")
		}
	})
}

func TestDifferService_NewDifferService(t *testing.T) {
	t.Run("with nil state", func(t *testing.T) {
		svc := NewDifferService(nil)
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
		if svc.state == nil {
			t.Fatal("expected initialized state")
		}
	})

	t.Run("with existing state", func(t *testing.T) {
		state := &repository.DeploymentState{
			Services: map[string]*entity.BizService{"test": {}},
		}
		svc := NewDifferService(state)
		if svc.state.Services["test"] == nil {
			t.Fatal("expected existing state to be preserved")
		}
	})
}

func TestDifferService_PlanISPs(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.ISP{
		"isp1": {Name: "isp1", Type: "cloudflare"},
		"isp2": {Name: "isp2", Type: "aliyun"},
	}

	svc.PlanISPs(plan, cfgMap, scope)

	if len(plan.Changes()) != 2 {
		t.Errorf("expected 2 changes, got %d", len(plan.Changes()))
	}
}

func TestDifferService_PlanISPs_Update(t *testing.T) {
	state := &repository.DeploymentState{
		ISPs: map[string]*entity.ISP{
			"isp1": {Name: "isp1", Type: "cloudflare", Services: []entity.ISPService{"server"}},
		},
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.ISP{
		"isp1": {Name: "isp1", Type: "cloudflare", Services: []entity.ISPService{"server", "domain"}},
	}

	svc.PlanISPs(plan, cfgMap, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
	if plan.Changes()[0].Type() != valueobject.ChangeTypeUpdate {
		t.Errorf("expected update change, got %v", plan.Changes()[0].Type())
	}
}

func TestDifferService_PlanISPs_Delete(t *testing.T) {
	state := &repository.DeploymentState{
		ISPs: map[string]*entity.ISP{
			"isp1": {Name: "isp1", Type: "cloudflare"},
		},
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.ISP{}

	svc.PlanISPs(plan, cfgMap, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
	if plan.Changes()[0].Type() != valueobject.ChangeTypeDelete {
		t.Errorf("expected delete change, got %v", plan.Changes()[0].Type())
	}
}

func TestISPEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *entity.ISP
		expected bool
	}{
		{
			name:     "equal ISPs",
			a:        &entity.ISP{Name: "isp1", Type: "cloudflare", Services: []entity.ISPService{"server", "domain"}},
			b:        &entity.ISP{Name: "isp1", Type: "cloudflare", Services: []entity.ISPService{"server", "domain"}},
			expected: true,
		},
		{
			name:     "different names",
			a:        &entity.ISP{Name: "isp1"},
			b:        &entity.ISP{Name: "isp2"},
			expected: false,
		},
		{
			name:     "different services length",
			a:        &entity.ISP{Name: "isp1", Services: []entity.ISPService{"server"}},
			b:        &entity.ISP{Name: "isp1", Services: []entity.ISPService{"server", "domain"}},
			expected: false,
		},
		{
			name:     "different services content",
			a:        &entity.ISP{Name: "isp1", Services: []entity.ISPService{"server", "domain"}},
			b:        &entity.ISP{Name: "isp1", Services: []entity.ISPService{"server", "dns"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ISPEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ISPEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestZoneEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *entity.Zone
		expected bool
	}{
		{
			name:     "equal zones",
			a:        &entity.Zone{Name: "zone1", ISP: "isp1", Region: "us-east"},
			b:        &entity.Zone{Name: "zone1", ISP: "isp1", Region: "us-east"},
			expected: true,
		},
		{
			name:     "different names",
			a:        &entity.Zone{Name: "zone1"},
			b:        &entity.Zone{Name: "zone2"},
			expected: false,
		},
		{
			name:     "different ISP",
			a:        &entity.Zone{Name: "zone1", ISP: "isp1"},
			b:        &entity.Zone{Name: "zone1", ISP: "isp2"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ZoneEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ZoneEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestDomainEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *entity.Domain
		expected bool
	}{
		{
			name:     "equal domains",
			a:        &entity.Domain{Name: "example.com", ISP: "isp1", Parent: ""},
			b:        &entity.Domain{Name: "example.com", ISP: "isp1", Parent: ""},
			expected: true,
		},
		{
			name:     "different names",
			a:        &entity.Domain{Name: "example.com"},
			b:        &entity.Domain{Name: "example.org"},
			expected: false,
		},
		{
			name:     "different parent",
			a:        &entity.Domain{Name: "sub.example.com", Parent: "example.com"},
			b:        &entity.Domain{Name: "sub.example.com", Parent: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DomainEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("DomainEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestServerEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *entity.Server
		expected bool
	}{
		{
			name:     "equal servers",
			a:        &entity.Server{Name: "srv1", Zone: "z1", ISP: "isp1", IP: entity.ServerIP{Public: "1.2.3.4", Private: "10.0.0.1"}},
			b:        &entity.Server{Name: "srv1", Zone: "z1", ISP: "isp1", IP: entity.ServerIP{Public: "1.2.3.4", Private: "10.0.0.1"}},
			expected: true,
		},
		{
			name:     "different names",
			a:        &entity.Server{Name: "srv1"},
			b:        &entity.Server{Name: "srv2"},
			expected: false,
		},
		{
			name:     "different public IP",
			a:        &entity.Server{Name: "srv1", IP: entity.ServerIP{Public: "1.2.3.4"}},
			b:        &entity.Server{Name: "srv1", IP: entity.ServerIP{Public: "1.2.3.5"}},
			expected: false,
		},
		{
			name: "equal servers with networks (same order)",
			a: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{
					{Name: "net1", Type: "bridge"},
					{Name: "net2", Type: "overlay"},
				},
			},
			b: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{
					{Name: "net1", Type: "bridge"},
					{Name: "net2", Type: "overlay"},
				},
			},
			expected: true,
		},
		{
			name: "equal servers with networks (different order)",
			a: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{
					{Name: "net1", Type: "bridge"},
					{Name: "net2", Type: "overlay"},
				},
			},
			b: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{
					{Name: "net2", Type: "overlay"},
					{Name: "net1", Type: "bridge"},
				},
			},
			expected: true,
		},
		{
			name: "different networks length",
			a: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1"}},
			},
			b: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1"}, {Name: "net2"}},
			},
			expected: false,
		},
		{
			name: "different network names",
			a: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1"}},
			},
			b: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net2"}},
			},
			expected: false,
		},
		{
			name: "different network type",
			a: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1", Type: "bridge"}},
			},
			b: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1", Type: "overlay"}},
			},
			expected: false,
		},
		{
			name: "different network driver",
			a: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1", Driver: "bridge"}},
			},
			b: &entity.Server{
				Name: "srv1", Zone: "z1",
				Networks: []entity.ServerNetwork{{Name: "net1", Driver: "macvlan"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ServerEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ServerEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRecordEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *entity.DNSRecord
		expected bool
	}{
		{
			name:     "equal records",
			a:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
			b:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
			expected: true,
		},
		{
			name:     "different value",
			a:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4"},
			b:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.5"},
			expected: false,
		},
		{
			name:     "different TTL",
			a:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
			b:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 600},
			expected: false,
		},
		{
			name:     "different line",
			a:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", Line: "default"},
			b:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", Line: "oversea"},
			expected: false,
		},
		{
			name:     "empty line equals default",
			a:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4"},
			b:        &entity.DNSRecord{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", Line: "default"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RecordEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("RecordEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestServiceEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *entity.BizService
		expected bool
	}{
		{
			name: "equal services",
			a: &entity.BizService{Name: "svc1", ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "app:v1",
				Ports: []entity.ServicePort{{Host: 8080}}, Env: map[string]valueobject.SecretRef{"KEY": *valueobject.NewSecretRefPlain("val")}},
			b: &entity.BizService{Name: "svc1", ServiceBase: entity.ServiceBase{Server: "srv1"}, Image: "app:v1",
				Ports: []entity.ServicePort{{Host: 8080}}, Env: map[string]valueobject.SecretRef{"KEY": *valueobject.NewSecretRefPlain("val")}},
			expected: true,
		},
		{
			name:     "different image",
			a:        &entity.BizService{Name: "svc1", Image: "app:v1"},
			b:        &entity.BizService{Name: "svc1", Image: "app:v2"},
			expected: false,
		},
		{
			name:     "different ports length",
			a:        &entity.BizService{Name: "svc1", Ports: []entity.ServicePort{{Host: 8080}}},
			b:        &entity.BizService{Name: "svc1", Ports: []entity.ServicePort{{Host: 8080}, {Host: 9090}}},
			expected: false,
		},
		{
			name:     "different env",
			a:        &entity.BizService{Name: "svc1", Env: map[string]valueobject.SecretRef{"KEY": *valueobject.NewSecretRefPlain("val1")}},
			b:        &entity.BizService{Name: "svc1", Env: map[string]valueobject.SecretRef{"KEY": *valueobject.NewSecretRefPlain("val2")}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ServiceEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ServiceEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestDifferService_PlanZones(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.Zone{
		"zone1": {Name: "zone1", ISP: "isp1"},
	}

	svc.PlanZones(plan, cfgMap, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
	if plan.Changes()[0].Type() != valueobject.ChangeTypeCreate {
		t.Errorf("expected create change, got %v", plan.Changes()[0].Type())
	}
}

func TestDifferService_PlanDomains(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.Domain{
		"example.com": {Name: "example.com", ISP: "isp1"},
	}

	svc.PlanDomains(plan, cfgMap, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
}

func TestDifferService_PlanRecords(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgRecords := []entity.DNSRecord{
		{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
	}

	svc.PlanRecords(plan, cfgRecords, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
}

func TestDifferService_PlanRecords_SameNameDiffLine(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgRecords := []entity.DNSRecord{
		{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
		{Domain: "example.com", Type: "A", Name: "www", Value: "5.6.7.8", TTL: 300, Line: "oversea"},
	}

	svc.PlanRecords(plan, cfgRecords, scope)

	if len(plan.Changes()) != 2 {
		t.Errorf("expected 2 changes (default + oversea), got %d", len(plan.Changes()))
	}
}

func TestDifferService_PlanRecords_StateLineMismatch(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	svc.state.Records["example.com:A:www:default"] = &entity.DNSRecord{
		Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300,
	}

	cfgRecords := []entity.DNSRecord{
		{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300, Line: "oversea"},
	}

	svc.PlanRecords(plan, cfgRecords, scope)

	if len(plan.Changes()) != 2 {
		t.Errorf("expected delete(default)+create(oversea) = 2 changes, got %d", len(plan.Changes()))
	}
}

// TestDifferService_PlanRecords_Synced5PartKey 验证：远端状态 key 按 5 段格式
// （含 EffectiveLine 与 Value，v1.0.27 起）构造时，与配置一致 → 无变更生成。
func TestDifferService_PlanRecords_Synced5PartKey(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	stateRec := &entity.DNSRecord{
		Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300,
	}
	svc.state.Records["example.com:A:www:"+stateRec.EffectiveLine()+":1.2.3.4"] = stateRec

	cfgRecords := []entity.DNSRecord{
		{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
	}

	svc.PlanRecords(plan, cfgRecords, scope)

	if len(plan.Changes()) != 0 {
		t.Errorf("expected 0 changes (records in sync), got %d", len(plan.Changes()))
	}
}

// TestDifferService_PlanRecords_LegacyKeyMismatchHealsToUpdate：state 键格式陈旧
// （v1.0.26 的 3 段/4 段键，缺少 Value 段）时，5 段精确匹配不中，但同名同线路分组
// 恰好 1↔1 → 收敛为原地 update，不再产生 v1.0.26 及之前的虚假 delete+create。
func TestDifferService_PlanRecords_3PartKeyMismatch(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	svc.state.Records["example.com:A:www"] = &entity.DNSRecord{
		Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300,
	}

	cfgRecords := []entity.DNSRecord{
		{Domain: "example.com", Type: "A", Name: "www", Value: "1.2.3.4", TTL: 300},
	}

	svc.PlanRecords(plan, cfgRecords, scope)

	if len(plan.Changes()) != 1 || plan.Changes()[0].Type() != valueobject.ChangeTypeUpdate {
		t.Errorf("expected legacy key mismatch to heal into 1 update, got %d changes", len(plan.Changes()))
	}
}

func TestDifferService_PlanServers(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.Server{
		"srv1": {Name: "srv1", Zone: "zone1"},
	}
	zoneMap := map[string]*entity.Zone{
		"zone1": {Name: "zone1"},
	}

	svc.PlanServers(plan, cfgMap, zoneMap, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
}

func TestDifferService_PlanServices(t *testing.T) {
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfgMap := map[string]*entity.BizService{
		"svc1": {Name: "svc1", ServiceBase: entity.ServiceBase{Server: "srv1"}},
	}
	serverMap := map[string]*entity.Server{
		"srv1": {Name: "srv1", Zone: "zone1"},
	}

	svc.PlanServices(plan, cfgMap, serverMap, scope)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
}

func TestDifferService_GetSetState(t *testing.T) {
	svc := NewDifferService(nil)

	state := &repository.DeploymentState{
		Servers: map[string]*entity.Server{"srv1": {Name: "srv1"}},
	}
	svc.SetState(state)

	retrieved := svc.GetState()
	if retrieved.Servers["srv1"] == nil {
		t.Error("expected state to be set and retrieved")
	}
}

func TestInfraServiceEquals_EasyTier(t *testing.T) {
	secret := *valueobject.NewSecretRefSecret("et_network_secret_prod")
	base := &entity.InfraService{
		Name:        "easytier-node5",
		Type:        entity.InfraServiceTypeEasyTier,
		Image:       "easytier:v2.6.4",
		ServiceBase: entity.ServiceBase{Server: "srv-node5"},
		EasyTierConfig: &entity.EasyTierConfig{
			Role:          "shared",
			VirtualIP:     "10.72.0.11/24",
			NetworkName:   "lite-mesh-prod",
			NetworkSecret: secret,
			Hostname:      "srv-node5",
			Listeners:     []string{"quic://0.0.0.0:11010"},
			Peers:         []string{"quic://103.236.57.87:11010", "quic://103.236.53.189:11010"},
			Flags: entity.EasyTierFlags{
				LatencyFirst: true,
				PrivateMode:  true,
			},
		},
	}

	clone := func() *entity.InfraService {
		et := *base.EasyTierConfig
		et.Listeners = append([]string(nil), base.EasyTierConfig.Listeners...)
		et.Peers = append([]string(nil), base.EasyTierConfig.Peers...)
		return &entity.InfraService{
			Name:           base.Name,
			Type:           base.Type,
			Image:          base.Image,
			ServiceBase:    base.ServiceBase,
			EasyTierConfig: &et,
		}
	}

	t.Run("equal", func(t *testing.T) {
		if !InfraServiceEquals(base, clone()) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("different virtual_ip", func(t *testing.T) {
		b := clone()
		b.EasyTierConfig.VirtualIP = "10.72.0.12/24"
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different virtual_ip)")
		}
	})

	t.Run("different peers", func(t *testing.T) {
		b := clone()
		b.EasyTierConfig.Peers = []string{"quic://103.236.53.189:11010"}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different peers)")
		}
	})

	t.Run("different flag", func(t *testing.T) {
		b := clone()
		b.EasyTierConfig.Flags.PrivateMode = false
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different flag)")
		}
	})

	t.Run("different network_secret", func(t *testing.T) {
		b := clone()
		b.EasyTierConfig.NetworkSecret = *valueobject.NewSecretRefSecret("other")
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different network_secret)")
		}
	})
}

func uptimeKumaTestConfigIf() *entity.UptimeKumaConfig {
	return &entity.UptimeKumaConfig{
		Admin: entity.UptimeKumaAdminConfig{
			Username: "admin",
			Password: *valueobject.NewSecretRefSecret("kuma_admin_password"),
		},
		DB: entity.UptimeKumaDBConfig{
			Type:     "mariadb",
			Hostname: "192.168.180.234",
			Port:     39675,
			Name:     "kuma_dev",
			Username: "kuma_dev",
			Password: *valueobject.NewSecretRefSecret("kuma_db_password"),
		},
		Gateways: []entity.UptimeKumaGatewayConfig{
			{Hostname: "uptime.env-dev.mesh.example.net", HTTPS: true, ContainerPort: 3001},
		},
		Sync: entity.UptimeKumaSyncConfig{
			Image:             "sync:dev",
			ReconcileInterval: "60s",
		},
		Notification: entity.UptimeKumaNotificationConfig{
			Enabled: true,
			URL:     "https://mc.example.com/webhook",
		},
		Monitors: []entity.UptimeKumaMonitorConfig{
			{Name: "m1", Type: "http", URL: "https://www.example.com"},
		},
	}
}

func TestInfraServiceEquals_UptimeKuma(t *testing.T) {
	base := &entity.InfraService{
		Name:             "uptime-kuma-node1",
		Type:             entity.InfraServiceTypeUptimeKuma,
		Image:            "kuma:2",
		ServiceBase:      entity.ServiceBase{Server: "srv-node1"},
		UptimeKumaConfig: uptimeKumaTestConfigIf(),
	}

	clone := func() *entity.InfraService {
		c := *base
		uk := *base.UptimeKumaConfig
		uk.Gateways = append([]entity.UptimeKumaGatewayConfig(nil), base.UptimeKumaConfig.Gateways...)
		uk.Monitors = append([]entity.UptimeKumaMonitorConfig(nil), base.UptimeKumaConfig.Monitors...)
		c.UptimeKumaConfig = &uk
		return &c
	}

	t.Run("equal", func(t *testing.T) {
		if !InfraServiceEquals(base, clone()) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("different db hostname triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.DB.Hostname = "192.168.180.235"
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different db hostname)")
		}
	})

	t.Run("different container_port triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Gateways[0].ContainerPort = 3005
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different container_port)")
		}
	})

	t.Run("different sync image triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Sync.Image = "sync:new"
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different sync image)")
		}
	})

	t.Run("different relogin_interval triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Sync.ReloginInterval = "0s"
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different relogin_interval)")
		}
	})

	t.Run("monitor added triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors = append(b.UptimeKumaConfig.Monitors,
			entity.UptimeKumaMonitorConfig{Name: "m2", Type: "http", URL: "https://api.example.com"})
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (monitor added)")
		}
	})

	t.Run("monitor removed triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors = nil
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (monitor removed)")
		}
	})

	t.Run("monitor replaced triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors = []entity.UptimeKumaMonitorConfig{
			{Name: "m2", Type: "ping", Hostname: "a"},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (monitor replaced)")
		}
	})

	t.Run("monitor field changed triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors[0].URL = "https://other.example.com"
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (monitor field changed)")
		}
	})

	t.Run("monitor extra field added triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors[0].Extra = map[string]any{"packetSize": 512}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (extra field added)")
		}
	})

	t.Run("monitor extra field value changed triggers rebuild", func(t *testing.T) {
		b1 := clone()
		b1.UptimeKumaConfig.Monitors[0].Extra = map[string]any{"packetSize": 512}
		b2 := clone()
		b2.UptimeKumaConfig.Monitors[0].Extra = map[string]any{"packetSize": 1024}
		if InfraServiceEquals(b1, b2) {
			t.Error("expected not equal (extra field value changed)")
		}
	})

	t.Run("monitor extra field removed triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors[0].Extra = map[string]any{"packetSize": 512}
		if InfraServiceEquals(base, b) {
			t.Fatal("precondition: extra added should differ")
		}
		b.UptimeKumaConfig.Monitors[0].Extra = nil
		if !InfraServiceEquals(base, b) {
			t.Error("expected equal (extra field removed)")
		}
	})

	t.Run("monitor accepted_statuscodes changed triggers rebuild", func(t *testing.T) {
		b := clone()
		b.UptimeKumaConfig.Monitors[0].AcceptedStatuscodes = []string{"200-299", "301-302"}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (accepted_statuscodes changed)")
		}
	})

	t.Run("monitor reordered triggers rebuild", func(t *testing.T) {
		base2 := clone()
		base2.UptimeKumaConfig.Monitors = append(base2.UptimeKumaConfig.Monitors,
			entity.UptimeKumaMonitorConfig{Name: "m2", Type: "port", Hostname: "up.example.com", Port: 443})
		b := &entity.InfraService{
			Name:             base2.Name,
			Type:             base2.Type,
			Image:            base2.Image,
			ServiceBase:      base2.ServiceBase,
			UptimeKumaConfig: base2.UptimeKumaConfig,
		}
		uk := *b.UptimeKumaConfig
		uk.Monitors = []entity.UptimeKumaMonitorConfig{
			uk.Monitors[1], uk.Monitors[0],
		}
		b.UptimeKumaConfig = &uk
		if InfraServiceEquals(base2, b) {
			t.Error("expected not equal (monitor reordered)")
		}
	})

	t.Run("monitors identical stays equal", func(t *testing.T) {
		base2 := clone()
		base2.UptimeKumaConfig.Monitors = append(base2.UptimeKumaConfig.Monitors,
			entity.UptimeKumaMonitorConfig{
				Name: "m2", Type: "port", Hostname: "up.example.com", Port: 443,
				AcceptedStatuscodes: []string{"200-299"},
			})
		b := &entity.InfraService{
			Name:             base2.Name,
			Type:             base2.Type,
			Image:            base2.Image,
			ServiceBase:      base2.ServiceBase,
			UptimeKumaConfig: base2.UptimeKumaConfig,
		}
		uk := *b.UptimeKumaConfig
		uk.Monitors = []entity.UptimeKumaMonitorConfig{
			uk.Monitors[0],
			{Name: "m2", Type: "port", Hostname: "up.example.com", Port: 443, AcceptedStatuscodes: []string{"200-299"}},
		}
		b.UptimeKumaConfig = &uk
		if !InfraServiceEquals(base2, b) {
			t.Error("expected equal (identical monitors)")
		}
	})

	t.Run("nil vs non-nil config", func(t *testing.T) {
		b := &entity.InfraService{
			Name:        "uptime-kuma-node1",
			Type:        entity.InfraServiceTypeUptimeKuma,
			Image:       "kuma:2",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		}
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (nil config)")
		}
	})
}

func TestInfraServiceEquals_GatewayBlacklist(t *testing.T) {
	base := &entity.InfraService{
		Name:        "gateway-node5",
		Type:        entity.InfraServiceTypeGateway,
		Image:       "gate:prod",
		ServiceBase: entity.ServiceBase{Server: "srv-node5"},
		GatewayBlacklist: &entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
			Whitelist:   []string{"203.0.113.10", "10.0.0.0/8"},
			SourceURL:   "https://mirror.example.com/list.txt",
		},
	}

	build := func(bl *entity.GatewayBlacklistConfig) *entity.InfraService {
		return &entity.InfraService{
			Name:             "gateway-node5",
			Type:             entity.InfraServiceTypeGateway,
			Image:            "gate:prod",
			ServiceBase:      entity.ServiceBase{Server: "srv-node5"},
			GatewayBlacklist: bl,
		}
	}

	t.Run("equal", func(t *testing.T) {
		b := build(&entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
			Whitelist:   []string{"203.0.113.10", "10.0.0.0/8"},
			SourceURL:   "https://mirror.example.com/list.txt",
		})
		if !InfraServiceEquals(base, b) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("equal whitelist order-insensitive", func(t *testing.T) {
		b := build(&entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
			Whitelist:   []string{"10.0.0.0/8", "203.0.113.10"},
			SourceURL:   "https://mirror.example.com/list.txt",
		})
		if !InfraServiceEquals(base, b) {
			t.Error("expected equal (whitelist order-insensitive), got not equal")
		}
	})

	t.Run("different enabled", func(t *testing.T) {
		b := build(&entity.GatewayBlacklistConfig{
			Enabled:     false,
			DefaultMode: "detect",
			Whitelist:   []string{"203.0.113.10", "10.0.0.0/8"},
			SourceURL:   "https://mirror.example.com/list.txt",
		})
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different enabled), got equal")
		}
	})

	t.Run("different default_mode", func(t *testing.T) {
		b := build(&entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "block",
			Whitelist:   []string{"203.0.113.10", "10.0.0.0/8"},
			SourceURL:   "https://mirror.example.com/list.txt",
		})
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different default_mode), got equal")
		}
	})

	t.Run("different whitelist", func(t *testing.T) {
		b := build(&entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
			Whitelist:   []string{"203.0.113.11", "10.0.0.0/8"},
			SourceURL:   "https://mirror.example.com/list.txt",
		})
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different whitelist), got equal")
		}
	})

	t.Run("different source_url", func(t *testing.T) {
		b := build(&entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
			Whitelist:   []string{"203.0.113.10", "10.0.0.0/8"},
			SourceURL:   "https://other.example.com/list.txt",
		})
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (different source_url), got equal")
		}
	})

	t.Run("nil vs non-nil config", func(t *testing.T) {
		b := build(nil)
		if InfraServiceEquals(base, b) {
			t.Error("expected not equal (nil config), got equal")
		}
	})
}

func TestServiceEquals_RouteBlacklistMode(t *testing.T) {
	build := func(mode string) *entity.BizService {
		return &entity.BizService{
			Name:        "web-service",
			Image:       "web:latest",
			ServiceBase: entity.ServiceBase{Server: "srv-node5"},
			Gateways: []entity.ServiceGatewayRoute{
				{Hostname: "web.mesh.example.net", ContainerPort: 80, HTTPS: true, BlacklistMode: mode},
			},
		}
	}

	t.Run("equal", func(t *testing.T) {
		if !ServiceEquals(build("detect"), build("detect")) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("different blacklist_mode", func(t *testing.T) {
		if ServiceEquals(build("detect"), build("disabled")) {
			t.Error("expected not equal (different blacklist_mode), got equal")
		}
	})

	t.Run("set vs unset", func(t *testing.T) {
		if ServiceEquals(build(""), build("disabled")) {
			t.Error("expected not equal (set vs unset blacklist_mode), got equal")
		}
	})
}

// TestServiceEquals_MeshForwardedForAutoTagNotEntityField 验证 v1.1 mesh 域名自动打标
// （forwarded_for: append，生成器内部规则）不影响 entity 级 diff：
// forwarded_for 不是 ServiceGatewayRoute 的用户字段，ServiceEquals/gatewayRouteEqual 对其
// 无感知——同一路由配置在打标前后 entity 相等（升级后 plan 不会对每个 biz 服务产生
// 伪 update），而真正的用户字段变化（如 allowed_ips）仍能被检测。生成产物的
// forwarded_for 变化属于 gate yaml 文件级差异，与所有 host 路由内容变更一致，
// 通过网关重新部署（显式选择网关或 --force）下发。
func TestServiceEquals_MeshForwardedForAutoTagNotEntityField(t *testing.T) {
	build := func(allowedIPs []string) *entity.BizService {
		return &entity.BizService{
			Name:        "app-uc-api",
			Image:       "uc:latest",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
			Gateways: []entity.ServiceGatewayRoute{
				{Hostname: "app-uc-api.node1.v-dev.mesh.example.net", ContainerPort: 80, HTTPS: true, AllowedIPs: allowedIPs},
			},
		}
	}

	t.Run("identical routes stay equal (auto-tag adds no entity diff)", func(t *testing.T) {
		// mesh host 路由在 yamlops 升级（自动打标）前后，用户配置完全相同：
		// entity 级必须判定相等，避免全量 biz 服务伪变更
		if !ServiceEquals(build(nil), build(nil)) {
			t.Error("expected equal (mesh route unchanged), got not equal")
		}
	})

	t.Run("real route field change still detected", func(t *testing.T) {
		if ServiceEquals(build(nil), build([]string{"10.71.0.0/24"})) {
			t.Error("expected not equal (allowed_ips changed), got equal")
		}
	})
}

func TestInfraServiceEquals_GatewayTrustedProxies(t *testing.T) {
	base := &entity.InfraService{
		Name:                  "gateway-node1",
		Type:                  entity.InfraServiceTypeGateway,
		Image:                 "gate:prod",
		ServiceBase:           entity.ServiceBase{Server: "srv-node1"},
		GatewayTrustedProxies: []string{"10.72.0.0/24", "10.71.0.0/24"},
	}

	build := func(tp []string) *entity.InfraService {
		return &entity.InfraService{
			Name:                  "gateway-node1",
			Type:                  entity.InfraServiceTypeGateway,
			Image:                 "gate:prod",
			ServiceBase:           entity.ServiceBase{Server: "srv-node1"},
			GatewayTrustedProxies: tp,
		}
	}

	t.Run("equal", func(t *testing.T) {
		if !InfraServiceEquals(base, build([]string{"10.72.0.0/24", "10.71.0.0/24"})) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("equal order-insensitive", func(t *testing.T) {
		if !InfraServiceEquals(base, build([]string{"10.71.0.0/24", "10.72.0.0/24"})) {
			t.Error("expected equal (order-insensitive), got not equal")
		}
	})

	t.Run("different entries", func(t *testing.T) {
		if InfraServiceEquals(base, build([]string{"10.72.0.0/24", "192.168.0.0/16"})) {
			t.Error("expected not equal (different entries), got equal")
		}
	})

	t.Run("set vs nil", func(t *testing.T) {
		if InfraServiceEquals(base, build(nil)) {
			t.Error("expected not equal (set vs nil), got equal")
		}
	})
}

func TestDifferService_PlanRecords_MultiValueSameGroup(t *testing.T) {
	// 同名同线路多值（@ TXT 挂 SPF + 站点验证）在空 state 下必须逐条 create，
	// 不得后写覆盖前写（v1.0.26 缺陷：dry-run 41/42 静默丢 SPF）
	svc := NewDifferService(nil)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfg := []entity.DNSRecord{
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "v=spf1 -all", TTL: 1},
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
		{Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "www", Value: "1.2.3.4", TTL: 600},
	}

	svc.PlanRecords(plan, cfg, scope)

	if len(plan.Changes()) != 3 {
		t.Fatalf("expected 3 creates, got %d", len(plan.Changes()))
	}
	values := map[string]bool{}
	for _, ch := range plan.Changes() {
		if ch.Type() != valueobject.ChangeTypeCreate {
			t.Errorf("expected create, got %v for %s", ch.Type(), ch.Name())
		}
		values[ch.NewState().(*entity.DNSRecord).Value] = true
	}
	if !values["v=spf1 -all"] || !values["google-site-verification=x"] {
		t.Error("expected both same-group TXT values planned, one was dropped")
	}
}

func TestDifferService_PlanRecords_MultiValueNoDrift(t *testing.T) {
	// live 已有同组两条多值记录且与 cfg 逐条一致 → 零变化（此前键不含 Value 时为盲区）
	state := &repository.DeploymentState{
		Records: map[string]*entity.DNSRecord{
			"example.com:TXT:@:default:v=spf1 -all":                {Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "v=spf1 -all", TTL: 300},
			"example.com:TXT:@:default:google-site-verification=x": {Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
		},
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfg := []entity.DNSRecord{
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "v=spf1 -all", TTL: 300},
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
	}

	svc.PlanRecords(plan, cfg, scope)

	if len(plan.Changes()) != 0 {
		t.Errorf("expected zero changes, got %d", len(plan.Changes()))
	}
}

func TestDifferService_PlanRecords_GroupAddKeepsExisting(t *testing.T) {
	// 组内已有 GSC、cfg 新增 SPF → 仅 create SPF，不动 GSC
	state := &repository.DeploymentState{
		Records: map[string]*entity.DNSRecord{
			"example.com:TXT:@:default:google-site-verification=x": {Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
		},
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfg := []entity.DNSRecord{
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "v=spf1 -all", TTL: 1},
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
	}

	svc.PlanRecords(plan, cfg, scope)

	if len(plan.Changes()) != 1 || plan.Changes()[0].Type() != valueobject.ChangeTypeCreate {
		t.Fatalf("expected 1 create, got %d changes", len(plan.Changes()))
	}
	if plan.Changes()[0].NewState().(*entity.DNSRecord).Value != "v=spf1 -all" {
		t.Errorf("expected SPF created, got %s", plan.Changes()[0].NewState().(*entity.DNSRecord).Value)
	}
}

func TestDifferService_PlanRecords_GroupRemoval(t *testing.T) {
	// live 有 SPF+GSC、cfg 仅留 GSC → 仅 delete SPF
	state := &repository.DeploymentState{
		Records: map[string]*entity.DNSRecord{
			"example.com:TXT:@:default:v=spf1 -all":                {Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "v=spf1 -all", TTL: 300},
			"example.com:TXT:@:default:google-site-verification=x": {Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
		},
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfg := []entity.DNSRecord{
		{Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "google-site-verification=x", TTL: 3600},
	}

	svc.PlanRecords(plan, cfg, scope)

	if len(plan.Changes()) != 1 || plan.Changes()[0].Type() != valueobject.ChangeTypeDelete {
		t.Fatalf("expected 1 delete, got %d changes", len(plan.Changes()))
	}
	if plan.Changes()[0].OldState().(*entity.DNSRecord).Value != "v=spf1 -all" {
		t.Errorf("expected SPF deleted, got %s", plan.Changes()[0].OldState().(*entity.DNSRecord).Value)
	}
}

func TestDifferService_PlanRecords_SingleValueChangeIsUpdate(t *testing.T) {
	// 同组恰 1 旧值 ↔ 1 新值 → 原地 update（保 IP 变更 in-place 语义，不得退化为 delete+create）
	for _, tc := range []struct {
		name       string
		state, cfg entity.DNSRecord
	}{
		{
			name:  "value change (IP migration)",
			state: entity.DNSRecord{Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "@", Value: "1.1.1.1", TTL: 600},
			cfg:   entity.DNSRecord{Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "@", Value: "2.2.2.2", TTL: 600},
		},
		{
			name:  "ttl-only change",
			state: entity.DNSRecord{Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "www", Value: "1.2.3.4", TTL: 600},
			cfg:   entity.DNSRecord{Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "www", Value: "1.2.3.4", TTL: 300},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &repository.DeploymentState{
				Records: map[string]*entity.DNSRecord{"k": &tc.state},
			}
			svc := NewDifferService(state)
			plan := valueobject.NewPlan()
			scope := valueobject.NewScope()

			svc.PlanRecords(plan, []entity.DNSRecord{tc.cfg}, scope)

			if len(plan.Changes()) != 1 || plan.Changes()[0].Type() != valueobject.ChangeTypeUpdate {
				t.Fatalf("expected 1 update, got %d changes", len(plan.Changes()))
			}
		})
	}
}

func TestDifferService_PlanRecords_StateOnlyDeleted(t *testing.T) {
	// live 独有记录（cfg 已移除）→ delete；live 与 cfg 一致的记录零变化
	state := &repository.DeploymentState{
		Records: map[string]*entity.DNSRecord{
			"example.com:A:www:default:1.2.3.4":      {Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "www", Value: "1.2.3.4", TTL: 600},
			"example.com:TXT:@:default:legacy-token": {Domain: "example.com", Type: entity.DNSRecordTypeTXT, Name: "@", Value: "legacy-token", TTL: 300},
		},
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()
	scope := valueobject.NewScope()

	cfg := []entity.DNSRecord{
		{Domain: "example.com", Type: entity.DNSRecordTypeA, Name: "www", Value: "1.2.3.4", TTL: 600},
	}

	svc.PlanRecords(plan, cfg, scope)

	if len(plan.Changes()) != 1 || plan.Changes()[0].Type() != valueobject.ChangeTypeDelete {
		t.Fatalf("expected 1 delete, got %d changes", len(plan.Changes()))
	}
	if plan.Changes()[0].OldState().(*entity.DNSRecord).Value != "legacy-token" {
		t.Errorf("expected legacy-token deleted, got %s", plan.Changes()[0].OldState().(*entity.DNSRecord).Value)
	}
}

// 逐条目 http→https 301 + 规范域收敛 + HSTS per-host（内部总体方案 W3-2 / D8）：
// ServiceGatewayRoute 三新字段参与 entity 级 diff——任一字段变化必须触发 update，
// 避免 userdata 配置改了跳转/HSTS 语义而 plan 看不见。
func TestServiceEquals_RedirectFields(t *testing.T) {
	tr := true
	off := "off"
	empty := ""

	build := func(redirectHTTPS *bool, redirectHost string, hsts *string) *entity.BizService {
		return &entity.BizService{
			Name:        "web-service",
			Image:       "web:latest",
			ServiceBase: entity.ServiceBase{Server: "srv-node5"},
			Gateways: []entity.ServiceGatewayRoute{
				{
					Hostname:      "www.mesh.example.net",
					ContainerPort: 80,
					HTTPS:         true,
					RedirectHTTPS: redirectHTTPS,
					RedirectHost:  redirectHost,
					HSTS:          hsts,
				},
			},
		}
	}

	t.Run("identical incl. all three fields", func(t *testing.T) {
		if !ServiceEquals(build(&tr, "mesh.example.net", &off), build(&tr, "mesh.example.net", &off)) {
			t.Error("expected equal, got not equal")
		}
	})

	t.Run("all three unset stays equal", func(t *testing.T) {
		if !ServiceEquals(build(nil, "", nil), build(nil, "", nil)) {
			t.Error("expected equal (both unset), got not equal")
		}
	})

	t.Run("redirect_https set vs unset detected", func(t *testing.T) {
		if ServiceEquals(build(nil, "", nil), build(&tr, "", nil)) {
			t.Error("expected not equal (redirect_https), got equal")
		}
	})

	t.Run("redirect_host change detected", func(t *testing.T) {
		if ServiceEquals(build(nil, "", nil), build(nil, "mesh.example.net", nil)) {
			t.Error("expected not equal (redirect_host), got equal")
		}
	})

	t.Run("hsts set vs unset detected", func(t *testing.T) {
		if ServiceEquals(build(nil, "", nil), build(nil, "", &off)) {
			t.Error("expected not equal (hsts), got equal")
		}
	})

	t.Run("hsts nil vs empty string detected", func(t *testing.T) {
		// 语义不同：nil = gate 默认值，"" = 不下发（entity 层必须能区分）
		if ServiceEquals(build(nil, "", nil), build(nil, "", &empty)) {
			t.Error("expected not equal (hsts nil vs empty string), got equal")
		}
	})
}

// ===== gate 安全加固新增字段的 diff 接线（backlog「yamlops 新字段透传补齐」） =====

// 网关级新字段（H1 逃生口 / L3 全局 Deprecated / H2 CRS pin 与明文镜像放行 / L6）参与 diff
func TestInfraServiceEquals_GatewaySecurityFields(t *testing.T) {
	tr := true
	fa := false
	pin := strings.Repeat("ab", 32)
	base := &entity.InfraService{
		Name:                         "gateway-node1",
		Type:                         entity.InfraServiceTypeGateway,
		Image:                        "gate:latest",
		ServiceBase:                  entity.ServiceBase{Server: "srv-node1"},
		GatewayStrictPathCheck:       &fa,
		GatewayWebSocketInsecureSkip: &tr,
		GatewayWAF: &entity.GatewayWAFConfig{
			Enabled:                    true,
			CRSPinnedSHA256:            pin,
			CRSMirrorAllowInsecureHTTP: &tr,
		},
		GatewayBlacklist: &entity.GatewayBlacklistConfig{
			Enabled:         true,
			AllowHTTPSource: &tr,
			MaxEntries:      300000,
		},
	}

	build := func(mutate func(s *entity.InfraService)) *entity.InfraService {
		s := &entity.InfraService{
			Name:                         "gateway-node1",
			Type:                         entity.InfraServiceTypeGateway,
			Image:                        "gate:latest",
			ServiceBase:                  entity.ServiceBase{Server: "srv-node1"},
			GatewayStrictPathCheck:       &fa,
			GatewayWebSocketInsecureSkip: &tr,
			GatewayWAF: &entity.GatewayWAFConfig{
				Enabled:                    true,
				CRSPinnedSHA256:            pin,
				CRSMirrorAllowInsecureHTTP: &tr,
			},
			GatewayBlacklist: &entity.GatewayBlacklistConfig{
				Enabled:         true,
				AllowHTTPSource: &tr,
				MaxEntries:      300000,
			},
		}
		if mutate != nil {
			mutate(s)
		}
		return s
	}

	if !InfraServiceEquals(base, build(nil)) {
		t.Error("expected equal baseline, got not equal")
	}

	tests := []struct {
		name   string
		mutate func(s *entity.InfraService)
	}{
		{"different strict_path_check", func(s *entity.InfraService) { s.GatewayStrictPathCheck = &tr }},
		{"strict_path_check nil vs set", func(s *entity.InfraService) { s.GatewayStrictPathCheck = nil }},
		{"different websocket_insecure_skip", func(s *entity.InfraService) { s.GatewayWebSocketInsecureSkip = &fa }},
		{"different crs_pinned_sha256", func(s *entity.InfraService) { s.GatewayWAF.CRSPinnedSHA256 = strings.Repeat("cd", 32) }},
		{"crs_pinned_sha256 nil vs set", func(s *entity.InfraService) {
			s.GatewayWAF.CRSMirrorAllowInsecureHTTP = nil
		}},
		{"different mirror_allow_insecure_http", func(s *entity.InfraService) { s.GatewayWAF.CRSMirrorAllowInsecureHTTP = &fa }},
		{"different allow_http_source", func(s *entity.InfraService) { s.GatewayBlacklist.AllowHTTPSource = &fa }},
		{"allow_http_source nil vs set", func(s *entity.InfraService) { s.GatewayBlacklist.AllowHTTPSource = nil }},
		{"different max_entries", func(s *entity.InfraService) { s.GatewayBlacklist.MaxEntries = 400000 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if InfraServiceEquals(base, build(tt.mutate)) {
				t.Errorf("expected not equal, got equal")
			}
		})
	}
}

// 业务路由级新字段参与 diff；含 auth 段整体比较回归（gatewayRouteEqual 曾漏比较 Auth 段，
// 改动 auth 不出 diff 的缺口随本次一并修复）
func TestServiceEquals_RouteAuthAndWSSFields(t *testing.T) {
	build := func(mutate func(route *entity.ServiceGatewayRoute)) *entity.BizService {
		route := entity.ServiceGatewayRoute{
			Hostname:      "gated.mesh.example.net",
			ContainerPort: 80,
			HTTPS:         true,
			Auth: &entity.RouteAuthConfig{
				Password:     valueobject.NewSecretRefSecret("gate_password"),
				TTL:          "24h",
				MaxFailures:  5,
				LockDuration: "15m",
			},
		}
		if mutate != nil {
			mutate(&route)
		}
		return &entity.BizService{
			Name:        "svc-demo",
			Image:       "demo:latest",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
			Gateways:    []entity.ServiceGatewayRoute{route},
		}
	}

	if !ServiceEquals(build(nil), build(nil)) {
		t.Error("expected equal baseline, got not equal")
	}

	tr := true
	tests := []struct {
		name   string
		mutate func(route *entity.ServiceGatewayRoute)
	}{
		// auth 段回归：以下任何一项变更都必须产生 diff
		{"different auth password ref", func(r *entity.ServiceGatewayRoute) {
			r.Auth.Password = valueobject.NewSecretRefSecret("other_password")
		}},
		{"different auth ttl", func(r *entity.ServiceGatewayRoute) { r.Auth.TTL = "12h" }},
		{"different auth max_failures", func(r *entity.ServiceGatewayRoute) { r.Auth.MaxFailures = 3 }},
		{"different auth lock_duration", func(r *entity.ServiceGatewayRoute) { r.Auth.LockDuration = "30m" }},
		{"auth nil vs set", func(r *entity.ServiceGatewayRoute) { r.Auth = nil }},
		// L3 per-host WSS 跳过校验
		{"web_socket_insecure_skip set", func(r *entity.ServiceGatewayRoute) { r.WebSocketInsecureSkip = &tr }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ServiceEquals(build(nil), build(tt.mutate)) {
				t.Errorf("expected not equal, got equal")
			}
		})
	}
}
