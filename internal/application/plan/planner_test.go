package plan

import (
	"testing"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestNewPlanner(t *testing.T) {
	cfg := &entity.Config{}
	planner := NewPlanner(WithConfig(cfg), WithEnv("dev"))

	if planner == nil {
		t.Fatal("expected non-nil planner")
	}
	if planner.env != "dev" {
		t.Errorf("expected env 'dev', got %s", planner.env)
	}
}

func TestNewPlanner_DefaultEnv(t *testing.T) {
	cfg := &entity.Config{}
	planner := NewPlanner(WithConfig(cfg))

	if planner.env != "dev" {
		t.Errorf("expected default env 'dev', got %s", planner.env)
	}
}

func TestNewPlannerWithState(t *testing.T) {
	cfg := &entity.Config{}
	st := &DeploymentState{
		Servers: map[string]*entity.Server{"srv1": {Name: "srv1"}},
	}
	planner := NewPlanner(WithConfig(cfg), WithEnv("prod"), WithState(st))

	if planner == nil {
		t.Fatal("expected non-nil planner")
	}
	if planner.GetState().Servers["srv1"] == nil {
		t.Error("expected state to be preserved")
	}
}

func TestPlanner_Plan(t *testing.T) {
	cfg := &entity.Config{
		ISPs: []entity.ISP{
			{Name: "isp1", Type: "cloudflare", Services: []entity.ISPService{"server"}},
		},
		Zones: []entity.Zone{
			{Name: "zone1", ISP: "isp1"},
		},
		Servers: []entity.Server{
			{Name: "srv1", Zone: "zone1"},
		},
	}

	planner := NewPlanner(WithConfig(cfg), WithEnv("dev"))
	plan, err := planner.Plan(nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Changes) == 0 {
		t.Error("expected some changes")
	}
}

func TestPlanner_Plan_WithScope(t *testing.T) {
	cfg := &entity.Config{
		Zones: []entity.Zone{
			{Name: "zone1"},
			{Name: "zone2"},
		},
	}

	planner := NewPlanner(WithConfig(cfg), WithEnv("dev"))
	scope := &valueobject.Scope{Zone: "zone1"}
	plan, err := planner.Plan(scope)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Scope.Zone != "zone1" {
		t.Errorf("expected scope zone 'zone1', got %s", plan.Scope.Zone)
	}
}

func TestPlanner_SetOutputDir(t *testing.T) {
	cfg := &entity.Config{}
	planner := NewPlanner(WithConfig(cfg), WithEnv("dev"))

	planner.SetOutputDir("/tmp/test")

	if planner.outputDir != "/tmp/test" {
		t.Errorf("expected outputDir '/tmp/test', got %s", planner.outputDir)
	}
}

func TestPlanner_GetConfig(t *testing.T) {
	cfg := &entity.Config{
		ISPs: []entity.ISP{{Name: "isp1"}},
	}
	planner := NewPlanner(WithConfig(cfg), WithEnv("dev"))

	retrieved := planner.GetConfig()

	if retrieved == nil {
		t.Fatal("expected non-nil config")
	}
	if len(retrieved.ISPs) != 1 {
		t.Errorf("expected 1 ISP, got %d", len(retrieved.ISPs))
	}
}

func TestPlanner_SetState(t *testing.T) {
	cfg := &entity.Config{}
	planner := NewPlanner(WithConfig(cfg), WithEnv("dev"))

	newState := &DeploymentState{
		Servers: map[string]*entity.Server{"srv1": {Name: "srv1"}},
	}
	planner.SetState(newState)

	if planner.GetState().Servers["srv1"] == nil {
		t.Error("expected state to be set")
	}
}
