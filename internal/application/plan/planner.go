package plan

import (
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/service"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/logger"
	"github.com/litelake/yamlops/internal/infrastructure/state"
)

type DeploymentState = repository.DeploymentState

type DeploymentGenerator interface {
	Generate(config *entity.Config) error
	SetOutputDir(dir string)
}

type Planner struct {
	config        *entity.Config
	differService *service.DifferService
	deployGen     DeploymentGenerator
	stateStore    *state.FileStore
	outputDir     string
	env           string
}

func NewPlanner(cfg *entity.Config, env string) *Planner {
	if env == "" {
		env = "dev"
	}
	return &Planner{
		config:        cfg,
		differService: service.NewDifferService(nil),
		deployGen:     NewDeploymentGeneratorAdapter(env, "deployments"),
		outputDir:     "deployments",
		env:           env,
	}
}

func NewPlannerWithState(cfg *entity.Config, st *DeploymentState, env string) *Planner {
	if env == "" {
		env = "dev"
	}
	return &Planner{
		config:        cfg,
		differService: service.NewDifferService(st),
		deployGen:     NewDeploymentGeneratorAdapter(env, "deployments"),
		outputDir:     "deployments",
		env:           env,
	}
}

func NewPlannerWithGenerator(cfg *entity.Config, st *DeploymentState, env string, gen DeploymentGenerator) *Planner {
	if env == "" {
		env = "dev"
	}
	return &Planner{
		config:        cfg,
		differService: service.NewDifferService(st),
		deployGen:     gen,
		outputDir:     "deployments",
		env:           env,
	}
}

func (p *Planner) SetOutputDir(dir string) {
	p.outputDir = dir
	p.deployGen.SetOutputDir(dir)
}

func (p *Planner) Plan(scope *valueobject.Scope) (*valueobject.Plan, error) {
	if scope == nil {
		scope = &valueobject.Scope{}
	}

	logger.Debug("starting plan generation", "env", p.env)

	plan := valueobject.NewPlanWithScope(scope)

	if !scope.HasAnyServiceSelection() {
		p.differService.PlanISPs(plan, p.config.GetISPMap(), scope)
		p.differService.PlanZones(plan, p.config.GetZoneMap(), scope)
		p.differService.PlanDomains(plan, p.config.GetDomainMap(), scope)
		p.differService.PlanRecords(plan, p.config.GetAllDNSRecords(), scope)
		p.differService.PlanCertificates(plan, p.config.GetCertificateMap(), scope)
		p.differService.PlanServers(plan, p.config.GetServerMap(), p.config.GetZoneMap(), scope)
	}

	if len(scope.Services) > 0 || (!scope.HasAnyServiceSelection() && !scope.DNSOnly) {
		p.differService.PlanServices(plan, p.config.GetServiceMap(), p.config.GetServerMap(), scope)
	}

	if len(scope.InfraServices) > 0 || (!scope.HasAnyServiceSelection() && !scope.DNSOnly) {
		p.differService.PlanInfraServices(plan, p.config.GetInfraServiceMap(), p.config.GetServerMap(), scope)
	}

	logger.Info("plan generated",
		"changes", len(plan.Changes),
		"env", p.env,
	)

	return plan, nil
}

func (p *Planner) GenerateDeployments() error {
	logger.Debug("generating deployments", "env", p.env, "output_dir", p.outputDir)
	err := p.deployGen.Generate(p.config)
	if err != nil {
		logger.Error("deployment generation failed", "error", err)
	} else {
		logger.Info("deployments generated", "output_dir", p.outputDir)
	}
	return err
}

func (p *Planner) GetConfig() *entity.Config {
	return p.config
}

func (p *Planner) GetState() *DeploymentState {
	return p.differService.GetState()
}

func (p *Planner) SetState(st *DeploymentState) {
	p.differService.SetState(st)
}

func (p *Planner) LoadStateFromFile(path string) error {
	p.stateStore = state.NewFileStore(path)
	st, err := p.stateStore.Load()
	if err != nil {
		return err
	}
	p.differService.SetState(st)
	return nil
}

func (p *Planner) SaveStateToFile(path string) error {
	p.stateStore = state.NewFileStore(path)
	return p.stateStore.Save(p.differService.GetState())
}
