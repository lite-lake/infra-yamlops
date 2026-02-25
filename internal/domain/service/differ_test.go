package service

import (
	"testing"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

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
			a: &entity.BizService{Name: "svc1", Server: "srv1", Image: "app:v1",
				Ports: []entity.ServicePort{{Host: 8080}}, Env: map[string]valueobject.SecretRef{"KEY": *valueobject.NewSecretRefPlain("val")}},
			b: &entity.BizService{Name: "svc1", Server: "srv1", Image: "app:v1",
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
		"svc1": {Name: "svc1", Server: "srv1"},
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
