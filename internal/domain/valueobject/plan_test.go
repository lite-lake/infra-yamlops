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
	scope := NewScope().WithZones([]string{"zone1"})
	plan := NewPlanWithScope(scope)

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Scope().Zones()) != 1 || plan.Scope().Zones()[0] != "zone1" {
		t.Errorf("expected scope zone 'zone1', got %v", plan.Scope().Zones())
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
