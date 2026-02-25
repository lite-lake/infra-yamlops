package plan

import (
	"context"

	"github.com/litelake/yamlops/internal/application/deployment"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/service"
	"github.com/litelake/yamlops/internal/domain/valueobject"
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
	stateRepo     repository.StateRepository
	outputDir     string
	env           string
}

type PlannerOption func(*Planner)

func WithConfig(cfg *entity.Config) PlannerOption {
	return func(p *Planner) { p.config = cfg }
}

func WithEnv(env string) PlannerOption {
	return func(p *Planner) {
		if env != "" {
			p.env = env
		}
	}
}

func WithState(st *DeploymentState) PlannerOption {
	return func(p *Planner) { p.differService = service.NewDifferService(st) }
}

func WithStateRepo(repo repository.StateRepository) PlannerOption {
	return func(p *Planner) { p.stateRepo = repo }
}

func WithGenerator(gen DeploymentGenerator) PlannerOption {
	return func(p *Planner) { p.deployGen = gen }
}

func WithOutputDir(dir string) PlannerOption {
	return func(p *Planner) { p.outputDir = dir }
}

func NewPlanner(opts ...PlannerOption) *Planner {
	p := &Planner{
		env:           "dev",
		outputDir:     "deployments",
		differService: service.NewDifferService(nil),
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.deployGen == nil {
		p.deployGen = deployment.NewGenerator(p.env, p.outputDir)
	}
	return p
}

func (p *Planner) SetOutputDir(dir string) {
	p.outputDir = dir
	p.deployGen.SetOutputDir(dir)
}

func (p *Planner) Plan(scope *valueobject.Scope) (*valueobject.Plan, error) {
	if scope == nil {
		scope = &valueobject.Scope{}
	}

	plan := valueobject.NewPlanWithScope(scope)

	if !scope.HasAnyServiceSelection() {
		p.differService.PlanISPs(plan, p.config.GetISPMap(), scope)
		p.differService.PlanZones(plan, p.config.GetZoneMap(), scope)
		p.differService.PlanDomains(plan, p.config.GetDomainMap(), scope)
		p.differService.PlanRecords(plan, p.config.GetAllDNSRecords(), scope)
		p.differService.PlanServers(plan, p.config.GetServerMap(), p.config.GetZoneMap(), scope)
	}

	if len(scope.Services()) > 0 || (!scope.HasAnyServiceSelection() && !scope.DNSOnly()) {
		p.differService.PlanServices(plan, p.config.GetServiceMap(), p.config.GetServerMap(), scope)
	}

	if len(scope.InfraServices()) > 0 || (!scope.HasAnyServiceSelection() && !scope.DNSOnly()) {
		p.differService.PlanInfraServices(plan, p.config.GetInfraServiceMap(), p.config.GetServerMap(), scope)
	}

	return plan, nil
}

func (p *Planner) GenerateDeployments() error {
	return p.deployGen.Generate(p.config)
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

func (p *Planner) LoadState(ctx context.Context) error {
	if p.stateRepo == nil {
		return nil
	}
	st, err := p.stateRepo.Load(ctx, p.env)
	if err != nil {
		return err
	}
	p.differService.SetState(st)
	return nil
}

func (p *Planner) SaveState(ctx context.Context) error {
	if p.stateRepo == nil {
		return nil
	}
	return p.stateRepo.Save(ctx, p.env, p.differService.GetState())
}
