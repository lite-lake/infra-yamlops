package valueobject

import (
	"testing"
)

func TestPlan_NewPlan(t *testing.T) {
	plan := NewPlan()

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Changes() == nil {
		t.Error("expected initialized changes slice")
	}
	if plan.Scope() == nil {
		t.Error("expected initialized scope")
	}
}

func TestPlan_NewPlanWithScope(t *testing.T) {
	scope := NewScope().WithZone("zone1")
	plan := NewPlanWithScope(scope)

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Scope().Zone() != "zone1" {
		t.Errorf("expected scope zone 'zone1', got %s", plan.Scope().Zone())
	}
}

func TestPlan_NewPlanWithScope_NilScope(t *testing.T) {
	plan := NewPlanWithScope(nil)

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Scope() == nil {
		t.Error("expected initialized scope")
	}
}

func TestPlan_AddChange(t *testing.T) {
	plan := NewPlan()
	change := NewChange(ChangeTypeCreate, "server", "srv1")

	plan.AddChange(change)

	if len(plan.Changes()) != 1 {
		t.Errorf("expected 1 change, got %d", len(plan.Changes()))
	}
}

func TestPlan_HasChanges(t *testing.T) {
	t.Run("with changes", func(t *testing.T) {
		plan := NewPlan()
		plan.AddChange(NewChange(ChangeTypeCreate, "server", "srv1"))

		if !plan.HasChanges() {
			t.Error("expected HasChanges to return true")
		}
	})

	t.Run("with noop only", func(t *testing.T) {
		plan := NewPlan()
		plan.AddChange(NewChange(ChangeTypeNoop, "server", "srv1"))

		if plan.HasChanges() {
			t.Error("expected HasChanges to return false")
		}
	})

	t.Run("empty", func(t *testing.T) {
		plan := NewPlan()

		if plan.HasChanges() {
			t.Error("expected HasChanges to return false")
		}
	})
}

func TestPlan_FilterByType(t *testing.T) {
	plan := NewPlan()
	plan.AddChange(NewChangeFull(ChangeTypeCreate, "server", "c1", nil, nil, nil, false))
	plan.AddChange(NewChangeFull(ChangeTypeUpdate, "server", "u1", nil, nil, nil, false))
	plan.AddChange(NewChangeFull(ChangeTypeCreate, "server", "c2", nil, nil, nil, false))

	creates := plan.FilterByType(ChangeTypeCreate)

	if len(creates) != 2 {
		t.Errorf("expected 2 create changes, got %d", len(creates))
	}
}

func TestPlan_FilterByEntity(t *testing.T) {
	plan := NewPlan()
	plan.AddChange(NewChangeFull(ChangeTypeCreate, "server", "s1", nil, nil, nil, false))
	plan.AddChange(NewChangeFull(ChangeTypeCreate, "service", "svc1", nil, nil, nil, false))
	plan.AddChange(NewChangeFull(ChangeTypeCreate, "server", "s2", nil, nil, nil, false))

	servers := plan.FilterByEntity("server")

	if len(servers) != 2 {
		t.Errorf("expected 2 server changes, got %d", len(servers))
	}
}

func TestScope_Matches(t *testing.T) {
	tests := []struct {
		name     string
		scope    *Scope
		zone     string
		server   string
		service  string
		domain   string
		expected bool
	}{
		{
			name:     "empty scope matches all",
			scope:    NewScope(),
			zone:     "zone1",
			server:   "srv1",
			service:  "svc1",
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "zone filter match",
			scope:    NewScope().WithZone("zone1"),
			zone:     "zone1",
			expected: true,
		},
		{
			name:     "zone filter no match",
			scope:    NewScope().WithZone("zone1"),
			zone:     "zone2",
			expected: false,
		},
		{
			name:     "server filter match",
			scope:    NewScope().WithServer("srv1"),
			server:   "srv1",
			expected: true,
		},
		{
			name:     "service filter match",
			scope:    NewScope().WithService("svc1"),
			service:  "svc1",
			expected: true,
		},
		{
			name:     "domain filter match",
			scope:    NewScope().WithDomain("example.com"),
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "multiple filters all match",
			scope:    NewScope().WithZone("zone1").WithServer("srv1"),
			zone:     "zone1",
			server:   "srv1",
			expected: true,
		},
		{
			name:     "multiple filters partial match",
			scope:    NewScope().WithZone("zone1").WithServer("srv1"),
			zone:     "zone1",
			server:   "srv2",
			expected: false,
		},
		{
			name:     "services slice match",
			scope:    NewScope().WithServices([]string{"svc1", "svc2"}),
			service:  "svc1",
			expected: true,
		},
		{
			name:     "services slice no match",
			scope:    NewScope().WithServices([]string{"svc1", "svc2"}),
			service:  "svc3",
			expected: false,
		},
		{
			name:     "services slice with zone match",
			scope:    NewScope().WithZone("zone1").WithServices([]string{"svc1"}),
			zone:     "zone1",
			service:  "svc1",
			expected: true,
		},
		{
			name:     "services slice with zone no match service",
			scope:    NewScope().WithZone("zone1").WithServices([]string{"svc1"}),
			zone:     "zone1",
			service:  "svc2",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scope.Matches(tt.zone, tt.server, tt.service, tt.domain)
			if result != tt.expected {
				t.Errorf("Matches() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		changeType ChangeType
		expected   string
	}{
		{ChangeTypeNoop, "NOOP"},
		{ChangeTypeCreate, "CREATE"},
		{ChangeTypeUpdate, "UPDATE"},
		{ChangeTypeDelete, "DELETE"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.changeType.String() != tt.expected {
				t.Errorf("String() = %s, expected %s", tt.changeType.String(), tt.expected)
			}
		})
	}
}
