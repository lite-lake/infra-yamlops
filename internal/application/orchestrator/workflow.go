package orchestrator

import (
	"context"
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/service"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
	"github.com/litelake/yamlops/internal/secrets"
)

type Workflow struct {
	env          string
	configDir    string
	loader       repository.ConfigLoader
	differ       *service.DifferService
	stateFetcher *StateFetcher
}

func NewWorkflow(env, configDir string) *Workflow {
	return &Workflow{
		env:          env,
		configDir:    configDir,
		loader:       persistence.NewConfigLoader(configDir),
		stateFetcher: NewStateFetcher(env, configDir),
	}
}

func (w *Workflow) Env() string { return w.env }

func (w *Workflow) LoadConfig(ctx context.Context) (*entity.Config, error) {
	cfg, err := w.loader.Load(ctx, w.env)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func (w *Workflow) LoadAndValidate(ctx context.Context) (*entity.Config, error) {
	cfg, err := w.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.loader.Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func (w *Workflow) ResolveSecrets(cfg *entity.Config) error {
	secretsList := make([]*entity.Secret, len(cfg.Secrets))
	for i := range cfg.Secrets {
		secretsList[i] = &cfg.Secrets[i]
	}
	resolver := secrets.NewSecretResolver(secretsList)
	return resolver.ResolveAll(cfg)
}

func (w *Workflow) CreatePlanner(cfg *entity.Config, outputDir string) *plan.Planner {
	planner := plan.NewPlanner(cfg, w.env)
	if outputDir != "" {
		planner.SetOutputDir(outputDir)
	}
	return planner
}

func (w *Workflow) Plan(ctx context.Context, outputDir string, scope *valueobject.Scope) (*valueobject.Plan, *entity.Config, error) {
	cfg, err := w.LoadAndValidate(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := w.ResolveSecrets(cfg); err != nil {
		return nil, nil, fmt.Errorf("resolve secrets: %w", err)
	}

	if err := w.GenerateDeployments(cfg, outputDir); err != nil {
		return nil, nil, fmt.Errorf("generate deployments: %w", err)
	}

	remoteState := w.stateFetcher.Fetch(ctx, cfg)
	planner := plan.NewPlannerWithState(cfg, remoteState, w.env)
	if outputDir != "" {
		planner.SetOutputDir(outputDir)
	}
	p, err := planner.Plan(scope)
	if err != nil {
		return nil, nil, fmt.Errorf("plan: %w", err)
	}
	return p, cfg, nil
}

func (w *Workflow) GenerateDeployments(cfg *entity.Config, outputDir string) error {
	planner := w.CreatePlanner(cfg, outputDir)
	return planner.GenerateDeployments()
}
