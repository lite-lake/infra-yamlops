package cli

import (
	"context"

	"github.com/litelake/yamlops/internal/application/orchestrator"
	"github.com/litelake/yamlops/internal/application/plan"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type Workflow struct {
	*orchestrator.Workflow
}

func NewWorkflow(env, configDir string) *Workflow {
	return &Workflow{
		Workflow: orchestrator.NewWorkflow(env, configDir),
	}
}

func (w *Workflow) CreatePlanner(cfg *entity.Config, outputDir string) *plan.Planner {
	return w.Workflow.CreatePlanner(cfg, outputDir)
}

func (w *Workflow) Plan(ctx context.Context, outputDir string, scope *valueobject.Scope) (*valueobject.Plan, *entity.Config, error) {
	return w.Workflow.Plan(ctx, outputDir, scope)
}

func (w *Workflow) GenerateDeployments(cfg *entity.Config, outputDir string) error {
	return w.Workflow.GenerateDeployments(cfg, outputDir)
}

func (w *Workflow) FetchRemoteState(ctx context.Context, cfg *entity.Config) *repository.DeploymentState {
	return w.Workflow.FetchRemoteState(ctx, cfg)
}

func (w *Workflow) LoadAndValidate(ctx context.Context) (*entity.Config, error) {
	return w.Workflow.LoadAndValidate(ctx)
}

func (w *Workflow) ResolveSecrets(cfg *entity.Config) error {
	return w.Workflow.ResolveSecrets(cfg)
}

func (w *Workflow) SaveState(ctx context.Context, cfg *entity.Config) error {
	return w.Workflow.SaveState(ctx, cfg)
}
