package valueobject

import (
	"testing"
)

func TestScope_NewScope(t *testing.T) {
	scope := NewScope()
	if scope == nil {
		t.Fatal("expected non-nil scope")
	}
	if !scope.IsEmpty() {
		t.Error("expected new scope to be empty")
	}
}

func TestScope_NewScopeFull(t *testing.T) {
	zones := []string{"z1", "z2"}
	servers := []string{"s1", "s2"}
	bizServices := []string{"biz1"}
	infraServices := []string{"infra1"}
	serviceTypes := []string{"biz", "infra"}
	domains := []string{"example.com"}
	dnsRecords := []string{"A:@", "CNAME:api"}
	forceDeploy := true

	scope := NewScopeFull(zones, servers, bizServices, infraServices, serviceTypes, domains, dnsRecords, forceDeploy)

	if scope == nil {
		t.Fatal("expected non-nil scope")
	}
	if len(scope.Zones()) != 2 {
		t.Errorf("expected 2 zones, got %d", len(scope.Zones()))
	}
	if len(scope.Servers()) != 2 {
		t.Errorf("expected 2 servers, got %d", len(scope.Servers()))
	}
	if len(scope.BizServices()) != 1 {
		t.Errorf("expected 1 biz service, got %d", len(scope.BizServices()))
	}
	if len(scope.InfraServices()) != 1 {
		t.Errorf("expected 1 infra service, got %d", len(scope.InfraServices()))
	}
	if len(scope.ServiceTypes()) != 2 {
		t.Errorf("expected 2 service types, got %d", len(scope.ServiceTypes()))
	}
	if len(scope.Domains()) != 1 {
		t.Errorf("expected 1 domain, got %d", len(scope.Domains()))
	}
	if len(scope.DNSRecords()) != 2 {
		t.Errorf("expected 2 dns records, got %d", len(scope.DNSRecords()))
	}
	if !scope.ForceDeploy() {
		t.Error("expected force deploy to be true")
	}
}

func TestScope_Getters(t *testing.T) {
	scope := NewScopeFull(
		[]string{"z1"},
		[]string{"s1"},
		[]string{"biz1"},
		[]string{"infra1"},
		[]string{"biz"},
		[]string{"example.com"},
		[]string{"A:@"},
		true,
	)

	if scope.Zones()[0] != "z1" {
		t.Errorf("expected zone 'z1', got %s", scope.Zones()[0])
	}
	if scope.Servers()[0] != "s1" {
		t.Errorf("expected server 's1', got %s", scope.Servers()[0])
	}
	if scope.BizServices()[0] != "biz1" {
		t.Errorf("expected biz service 'biz1', got %s", scope.BizServices()[0])
	}
	if scope.InfraServices()[0] != "infra1" {
		t.Errorf("expected infra service 'infra1', got %s", scope.InfraServices()[0])
	}
	if scope.ServiceTypes()[0] != "biz" {
		t.Errorf("expected service type 'biz', got %s", scope.ServiceTypes()[0])
	}
	if scope.Domains()[0] != "example.com" {
		t.Errorf("expected domain 'example.com', got %s", scope.Domains()[0])
	}
	if scope.DNSRecords()[0] != "A:@" {
		t.Errorf("expected dns record 'A:@', got %s", scope.DNSRecords()[0])
	}
	if !scope.ForceDeploy() {
		t.Error("expected force deploy to be true")
	}
}

func TestScope_WithZones(t *testing.T) {
	scope := NewScope().WithZones([]string{"z1", "z2"})
	if len(scope.Zones()) != 2 {
		t.Errorf("expected 2 zones, got %d", len(scope.Zones()))
	}
	if scope.Zones()[0] != "z1" || scope.Zones()[1] != "z2" {
		t.Errorf("expected zones [z1, z2], got %v", scope.Zones())
	}
}

func TestScope_WithServers(t *testing.T) {
	scope := NewScope().WithServers([]string{"s1", "s2"})
	if len(scope.Servers()) != 2 {
		t.Errorf("expected 2 servers, got %d", len(scope.Servers()))
	}
}

func TestScope_WithBizServices(t *testing.T) {
	scope := NewScope().WithBizServices([]string{"biz1"})
	if len(scope.BizServices()) != 1 {
		t.Errorf("expected 1 biz service, got %d", len(scope.BizServices()))
	}
}

func TestScope_WithInfraServices(t *testing.T) {
	scope := NewScope().WithInfraServices([]string{"infra1"})
	if len(scope.InfraServices()) != 1 {
		t.Errorf("expected 1 infra service, got %d", len(scope.InfraServices()))
	}
}

func TestScope_WithServiceTypes(t *testing.T) {
	scope := NewScope().WithServiceTypes([]string{"biz", "infra"})
	if len(scope.ServiceTypes()) != 2 {
		t.Errorf("expected 2 service types, got %d", len(scope.ServiceTypes()))
	}
}

func TestScope_WithDomains(t *testing.T) {
	scope := NewScope().WithDomains([]string{"example.com", "test.com"})
	if len(scope.Domains()) != 2 {
		t.Errorf("expected 2 domains, got %d", len(scope.Domains()))
	}
}

func TestScope_WithDNSRecords(t *testing.T) {
	scope := NewScope().WithDNSRecords([]string{"A:@", "CNAME:api"})
	if len(scope.DNSRecords()) != 2 {
		t.Errorf("expected 2 dns records, got %d", len(scope.DNSRecords()))
	}
}

func TestScope_WithForceDeploy(t *testing.T) {
	scope := NewScope().WithForceDeploy(true)
	if !scope.ForceDeploy() {
		t.Error("expected force deploy to be true")
	}
}

func TestScope_WithServiceType(t *testing.T) {
	scope := NewScope().WithServiceType("biz").WithServiceType("infra")
	if len(scope.ServiceTypes()) != 2 {
		t.Errorf("expected 2 service types, got %d", len(scope.ServiceTypes()))
	}
	if scope.ServiceTypes()[0] != "biz" || scope.ServiceTypes()[1] != "infra" {
		t.Errorf("expected service types [biz, infra], got %v", scope.ServiceTypes())
	}
}

func TestScope_ImmutableCopy(t *testing.T) {
	original := NewScope().WithZones([]string{"z1"})
	modified := original.WithZones([]string{"z2"})

	// Original should not be modified
	if original.Zones()[0] != "z1" {
		t.Errorf("expected original zone 'z1', got %s", original.Zones()[0])
	}
	// Modified should have new value
	if modified.Zones()[0] != "z2" {
		t.Errorf("expected modified zone 'z2', got %s", modified.Zones()[0])
	}
}

func TestScope_Matches(t *testing.T) {
	tests := []struct {
		name         string
		scope        *Scope
		zone         string
		server       string
		bizService   string
		infraService string
		domain       string
		record       string
		expected     bool
	}{
		{
			name:     "empty scope matches all",
			scope:    NewScope(),
			zone:     "z1",
			server:   "s1",
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "zone filter match",
			scope:    NewScope().WithZones([]string{"z1"}),
			zone:     "z1",
			expected: true,
		},
		{
			name:     "zone filter no match",
			scope:    NewScope().WithZones([]string{"z1"}),
			zone:     "z2",
			expected: false,
		},
		{
			name:     "server filter match",
			scope:    NewScope().WithServers([]string{"s1"}),
			server:   "s1",
			expected: true,
		},
		{
			name:     "server filter no match",
			scope:    NewScope().WithServers([]string{"s1"}),
			server:   "s2",
			expected: false,
		},
		{
			name:       "biz service filter match",
			scope:      NewScope().WithBizServices([]string{"biz1"}),
			bizService: "biz1",
			expected:   true,
		},
		{
			name:       "biz service filter no match",
			scope:      NewScope().WithBizServices([]string{"biz1"}),
			bizService: "biz2",
			expected:   false,
		},
		{
			name:         "infra service filter match",
			scope:        NewScope().WithInfraServices([]string{"infra1"}),
			infraService: "infra1",
			expected:     true,
		},
		{
			name:         "infra service filter no match",
			scope:        NewScope().WithInfraServices([]string{"infra1"}),
			infraService: "infra2",
			expected:     false,
		},
		{
			name:     "domain filter match",
			scope:    NewScope().WithDomains([]string{"example.com"}),
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "domain filter no match",
			scope:    NewScope().WithDomains([]string{"example.com"}),
			domain:   "test.com",
			expected: false,
		},
		{
			name:     "record filter match",
			scope:    NewScope().WithDNSRecords([]string{"A:@", "CNAME:api"}),
			record:   "A:@",
			expected: true,
		},
		{
			name:     "record filter no match",
			scope:    NewScope().WithDNSRecords([]string{"A:@", "CNAME:api"}),
			record:   "A:www",
			expected: false,
		},
		{
			name:     "multiple filters all match",
			scope:    NewScope().WithZones([]string{"z1"}).WithServers([]string{"s1"}),
			zone:     "z1",
			server:   "s1",
			expected: true,
		},
		{
			name:     "multiple filters partial match",
			scope:    NewScope().WithZones([]string{"z1"}).WithServers([]string{"s1"}),
			zone:     "z1",
			server:   "s2",
			expected: false,
		},
		{
			name:         "all filters match",
			scope:        NewScope().WithZones([]string{"z1"}).WithServers([]string{"s1"}).WithBizServices([]string{"biz1"}),
			zone:         "z1",
			server:       "s1",
			bizService:   "biz1",
			infraService: "infra1",
			domain:       "example.com",
			record:       "A:@",
			expected:     true,
		},
		{
			name:     "multi-value zone match",
			scope:    NewScope().WithZones([]string{"z1", "z2"}),
			zone:     "z2",
			expected: true,
		},
		{
			name:     "multi-value zone no match",
			scope:    NewScope().WithZones([]string{"z1", "z2"}),
			zone:     "z3",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scope.Matches(tt.zone, tt.server, tt.bizService, tt.infraService, tt.domain, tt.record)
			if result != tt.expected {
				t.Errorf("Matches() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestScope_MatchesZone(t *testing.T) {
	scope := NewScope().WithZones([]string{"z1", "z2"})

	if !scope.MatchesZone("z1") {
		t.Error("expected MatchesZone('z1') to be true")
	}
	if !scope.MatchesZone("z2") {
		t.Error("expected MatchesZone('z2') to be true")
	}
	if scope.MatchesZone("z3") {
		t.Error("expected MatchesZone('z3') to be false")
	}

	// Empty scope matches all
	emptyScope := NewScope()
	if !emptyScope.MatchesZone("any") {
		t.Error("expected empty scope to match any zone")
	}
}

func TestScope_MatchesServer(t *testing.T) {
	scope := NewScope().WithServers([]string{"s1"})

	if !scope.MatchesServer("s1") {
		t.Error("expected MatchesServer('s1') to be true")
	}
	if scope.MatchesServer("s2") {
		t.Error("expected MatchesServer('s2') to be false")
	}
}

func TestScope_MatchesBizService(t *testing.T) {
	scope := NewScope().WithBizServices([]string{"biz1", "biz2"})

	if !scope.MatchesBizService("biz1") {
		t.Error("expected MatchesBizService('biz1') to be true")
	}
	if scope.MatchesBizService("biz3") {
		t.Error("expected MatchesBizService('biz3') to be false")
	}
}

func TestScope_MatchesInfraService(t *testing.T) {
	scope := NewScope().WithInfraServices([]string{"infra1"})

	if !scope.MatchesInfraService("infra1") {
		t.Error("expected MatchesInfraService('infra1') to be true")
	}
	if scope.MatchesInfraService("infra2") {
		t.Error("expected MatchesInfraService('infra2') to be false")
	}
}

func TestScope_MatchesDomain(t *testing.T) {
	scope := NewScope().WithDomains([]string{"example.com"})

	if !scope.MatchesDomain("example.com") {
		t.Error("expected MatchesDomain('example.com') to be true")
	}
	if scope.MatchesDomain("test.com") {
		t.Error("expected MatchesDomain('test.com') to be false")
	}
}

func TestScope_MatchesDNSRecord(t *testing.T) {
	scope := NewScope().WithDNSRecords([]string{"A:@", "CNAME:api"})

	if !scope.MatchesDNSRecord("A:@") {
		t.Error("expected MatchesDNSRecord('A:@') to be true")
	}
	if !scope.MatchesDNSRecord("CNAME:api") {
		t.Error("expected MatchesDNSRecord('CNAME:api') to be true")
	}
	if scope.MatchesDNSRecord("A:www") {
		t.Error("expected MatchesDNSRecord('A:www') to be false")
	}
}

func TestScope_ShouldProcessBiz(t *testing.T) {
	tests := []struct {
		name         string
		serviceTypes []string
		expected     bool
	}{
		{"nil types", nil, true},
		{"empty types", []string{}, true},
		{"biz only", []string{"biz"}, true},
		{"infra only", []string{"infra"}, false},
		{"both types", []string{"biz", "infra"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := NewScope().WithServiceTypes(tt.serviceTypes)
			if scope.ShouldProcessBiz() != tt.expected {
				t.Errorf("ShouldProcessBiz() = %v, expected %v", scope.ShouldProcessBiz(), tt.expected)
			}
		})
	}
}

func TestScope_ShouldProcessInfra(t *testing.T) {
	tests := []struct {
		name         string
		serviceTypes []string
		expected     bool
	}{
		{"nil types", nil, true},
		{"empty types", []string{}, true},
		{"biz only", []string{"biz"}, false},
		{"infra only", []string{"infra"}, true},
		{"both types", []string{"biz", "infra"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := NewScope().WithServiceTypes(tt.serviceTypes)
			if scope.ShouldProcessInfra() != tt.expected {
				t.Errorf("ShouldProcessInfra() = %v, expected %v", scope.ShouldProcessInfra(), tt.expected)
			}
		})
	}
}

func TestScope_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		scope    *Scope
		expected bool
	}{
		{"empty", NewScope(), true},
		{"with zones", NewScope().WithZones([]string{"z1"}), false},
		{"with servers", NewScope().WithServers([]string{"s1"}), false},
		{"with biz services", NewScope().WithBizServices([]string{"biz1"}), false},
		{"with infra services", NewScope().WithInfraServices([]string{"infra1"}), false},
		{"with service types", NewScope().WithServiceTypes([]string{"biz"}), false},
		{"with domains", NewScope().WithDomains([]string{"example.com"}), false},
		{"with dns records", NewScope().WithDNSRecords([]string{"A:@"}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.scope.IsEmpty() != tt.expected {
				t.Errorf("IsEmpty() = %v, expected %v", tt.scope.IsEmpty(), tt.expected)
			}
		})
	}
}

func TestScope_HasServices(t *testing.T) {
	tests := []struct {
		name     string
		scope    *Scope
		expected bool
	}{
		{"empty", NewScope(), false},
		{"with biz services", NewScope().WithBizServices([]string{"biz1"}), true},
		{"with infra services", NewScope().WithInfraServices([]string{"infra1"}), true},
		{"with service types", NewScope().WithServiceTypes([]string{"biz"}), true},
		{"with domains only", NewScope().WithDomains([]string{"example.com"}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.scope.HasServices() != tt.expected {
				t.Errorf("HasServices() = %v, expected %v", tt.scope.HasServices(), tt.expected)
			}
		})
	}
}

func TestScope_HasDNSResources(t *testing.T) {
	tests := []struct {
		name     string
		scope    *Scope
		expected bool
	}{
		{"empty", NewScope(), false},
		{"with domains", NewScope().WithDomains([]string{"example.com"}), true},
		{"with dns records", NewScope().WithDNSRecords([]string{"A:@"}), true},
		{"with both", NewScope().WithDomains([]string{"example.com"}).WithDNSRecords([]string{"A:@"}), true},
		{"with servers only", NewScope().WithServers([]string{"s1"}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.scope.HasDNSResources() != tt.expected {
				t.Errorf("HasDNSResources() = %v, expected %v", tt.scope.HasDNSResources(), tt.expected)
			}
		})
	}
}

func TestScope_IsServiceOnly(t *testing.T) {
	tests := []struct {
		name     string
		scope    *Scope
		expected bool
	}{
		{"empty", NewScope(), false},
		{"service only", NewScope().WithBizServices([]string{"biz1"}), true},
		{"dns only", NewScope().WithDomains([]string{"example.com"}), false},
		{"both", NewScope().WithBizServices([]string{"biz1"}).WithDomains([]string{"example.com"}), false},
		{"service types only", NewScope().WithServiceTypes([]string{"biz"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.scope.IsServiceOnly() != tt.expected {
				t.Errorf("IsServiceOnly() = %v, expected %v", tt.scope.IsServiceOnly(), tt.expected)
			}
		})
	}
}

func TestScope_IsDNSOnly(t *testing.T) {
	tests := []struct {
		name     string
		scope    *Scope
		expected bool
	}{
		{"empty", NewScope(), false},
		{"dns only", NewScope().WithDomains([]string{"example.com"}), true},
		{"service only", NewScope().WithBizServices([]string{"biz1"}), false},
		{"both", NewScope().WithBizServices([]string{"biz1"}).WithDomains([]string{"example.com"}), false},
		{"dns records only", NewScope().WithDNSRecords([]string{"A:@"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.scope.IsDNSOnly() != tt.expected {
				t.Errorf("IsDNSOnly() = %v, expected %v", tt.scope.IsDNSOnly(), tt.expected)
			}
		})
	}
}

func TestScope_Equals(t *testing.T) {
	tests := []struct {
		name     string
		scope1   *Scope
		scope2   *Scope
		expected bool
	}{
		{
			name:     "both empty",
			scope1:   NewScope(),
			scope2:   NewScope(),
			expected: true,
		},
		{
			name:     "same values",
			scope1:   NewScope().WithZones([]string{"z1"}).WithServers([]string{"s1"}),
			scope2:   NewScope().WithZones([]string{"z1"}).WithServers([]string{"s1"}),
			expected: true,
		},
		{
			name:     "different zones",
			scope1:   NewScope().WithZones([]string{"z1"}),
			scope2:   NewScope().WithZones([]string{"z2"}),
			expected: false,
		},
		{
			name:     "different force deploy",
			scope1:   NewScope().WithForceDeploy(true),
			scope2:   NewScope().WithForceDeploy(false),
			expected: false,
		},
		{
			name:     "nil vs empty slice",
			scope1:   NewScope(),
			scope2:   NewScope().WithZones([]string{}),
			expected: true, // Both are effectively empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.scope1.Equals(tt.scope2) != tt.expected {
				t.Errorf("Equals() = %v, expected %v", tt.scope1.Equals(tt.scope2), tt.expected)
			}
		})
	}

	// Test nil comparison
	scope := NewScope()
	if scope.Equals(nil) {
		t.Error("expected Equals(nil) to be false")
	}
}

func TestScope_Clone(t *testing.T) {
	original := NewScopeFull(
		[]string{"z1"},
		[]string{"s1"},
		[]string{"biz1"},
		[]string{"infra1"},
		[]string{"biz"},
		[]string{"example.com"},
		[]string{"A:@"},
		true,
	)

	cloned := original.Clone()

	// Verify all values are copied
	if !original.Equals(cloned) {
		t.Error("expected cloned scope to equal original")
	}

	// Verify independence (modifying clone doesn't affect original)
	cloned.zones[0] = "z2"
	if original.Zones()[0] == "z2" {
		t.Error("expected original to be independent of clone")
	}
}

func TestScope_CopySlice(t *testing.T) {
	// Test nil slice
	if copySlice(nil) != nil {
		t.Error("expected copySlice(nil) to return nil")
	}

	// Test non-nil slice
	src := []string{"a", "b", "c"}
	dst := copySlice(src)

	if len(dst) != len(src) {
		t.Errorf("expected len %d, got %d", len(src), len(dst))
	}

	// Verify independence
	dst[0] = "x"
	if src[0] == "x" {
		t.Error("expected source to be independent of copy")
	}
}

func TestScope_Contains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"nil slice", nil, "a", false},
		{"empty slice", []string{}, "a", false},
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if contains(tt.slice, tt.item) != tt.expected {
				t.Errorf("contains() = %v, expected %v", contains(tt.slice, tt.item), tt.expected)
			}
		})
	}
}

func TestScope_SlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"nil vs empty", nil, []string{}, true},
		{"both empty", []string{}, []string{}, true},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, false},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different values", []string{"a", "b"}, []string{"a", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if slicesEqual(tt.a, tt.b) != tt.expected {
				t.Errorf("slicesEqual() = %v, expected %v", slicesEqual(tt.a, tt.b), tt.expected)
			}
		})
	}
}
