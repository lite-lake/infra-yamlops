package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lite-lake/infra-yamlops/internal/application/plan"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"github.com/lite-lake/infra-yamlops/internal/domain/service"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/secrets"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/state"
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
	opts := []plan.PlannerOption{
		plan.WithConfig(cfg),
		plan.WithEnv(w.env),
	}
	if outputDir != "" {
		opts = append(opts, plan.WithOutputDir(outputDir))
	}
	return plan.NewPlanner(opts...)
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
	opts := []plan.PlannerOption{
		plan.WithConfig(cfg),
		plan.WithEnv(w.env),
		plan.WithState(remoteState),
	}
	if outputDir != "" {
		opts = append(opts, plan.WithOutputDir(outputDir))
	}
	planner := plan.NewPlanner(opts...)
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

func (w *Workflow) FetchRemoteState(ctx context.Context, cfg *entity.Config) *repository.DeploymentState {
	return w.stateFetcher.Fetch(ctx, cfg)
}

func (w *Workflow) SaveState(ctx context.Context, cfg *entity.Config) error {
	stateDir := filepath.Join(w.configDir, constants.StateDir)
	if err := os.MkdirAll(stateDir, constants.DirPermissionOwner); err != nil {
		return fmt.Errorf("creating state directory %s: %w", stateDir, err)
	}

	statePath := filepath.Join(stateDir, fmt.Sprintf(constants.StateFileFormat, w.env))
	store := state.NewFileStore(statePath)

	state := repository.NewDeploymentState()

	for i := range cfg.Services {
		state.Services[cfg.Services[i].Name] = &cfg.Services[i]
	}
	for i := range cfg.InfraServices {
		state.InfraServices[cfg.InfraServices[i].Name] = &cfg.InfraServices[i]
	}
	for i := range cfg.Servers {
		state.Servers[cfg.Servers[i].Name] = &cfg.Servers[i]
	}
	for i := range cfg.Zones {
		state.Zones[cfg.Zones[i].Name] = &cfg.Zones[i]
	}
	for i := range cfg.Domains {
		state.Domains[cfg.Domains[i].Name] = &cfg.Domains[i]
		for _, r := range cfg.Domains[i].FlattenRecords() {
			record := r
			key := fmt.Sprintf("%s:%s:%s", record.Domain, record.Type, record.Name)
			state.Records[key] = &record
		}
	}
	for i := range cfg.ISPs {
		state.ISPs[cfg.ISPs[i].Name] = &cfg.ISPs[i]
	}

	if err := store.Save(ctx, w.env, state); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	return nil
}
