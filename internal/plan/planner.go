package plan

import (
	"fmt"
	"os"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/service"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"gopkg.in/yaml.v3"
)

type DeploymentState = repository.DeploymentState

type Planner struct {
	config         *entity.Config
	plannerService *service.PlannerService
	deployGen      *deploymentGenerator
	outputDir      string
	env            string
}

func NewPlanner(cfg *entity.Config, env string) *Planner {
	if env == "" {
		env = "dev"
	}
	return &Planner{
		config:         cfg,
		plannerService: service.NewPlannerService(nil),
		deployGen:      newDeploymentGenerator(env, "deployments"),
		outputDir:      "deployments",
		env:            env,
	}
}

func NewPlannerWithState(cfg *entity.Config, state *DeploymentState, env string) *Planner {
	if env == "" {
		env = "dev"
	}
	return &Planner{
		config:         cfg,
		plannerService: service.NewPlannerService(state),
		deployGen:      newDeploymentGenerator(env, "deployments"),
		outputDir:      "deployments",
		env:            env,
	}
}

func (p *Planner) SetOutputDir(dir string) {
	p.outputDir = dir
	p.deployGen.outputDir = dir
}

func (p *Planner) Plan(scope *valueobject.Scope) (*valueobject.Plan, error) {
	if scope == nil {
		scope = &valueobject.Scope{}
	}

	plan := valueobject.NewPlanWithScope(scope)

	if !scope.HasAnyServiceSelection() {
		p.plannerService.PlanISPs(plan, p.config.GetISPMap(), scope)
		p.plannerService.PlanZones(plan, p.config.GetZoneMap(), scope)
		p.plannerService.PlanDomains(plan, p.config.GetDomainMap(), scope)
		p.plannerService.PlanRecords(plan, p.config.GetAllDNSRecords(), scope)
		p.plannerService.PlanCertificates(plan, p.config.GetCertificateMap(), scope)
		p.plannerService.PlanRegistries(plan, p.config.GetRegistryMap(), scope)
		p.plannerService.PlanServers(plan, p.config.GetServerMap(), p.config.GetZoneMap(), scope)
	}

	if len(scope.Services) > 0 || !scope.HasAnyServiceSelection() {
		p.plannerService.PlanServices(plan, p.config.GetServiceMap(), p.config.GetServerMap(), scope)
	}

	if len(scope.InfraServices) > 0 || !scope.HasAnyServiceSelection() {
		p.plannerService.PlanInfraServices(plan, p.config.GetInfraServiceMap(), p.config.GetServerMap(), scope)
	}

	return plan, nil
}

func (p *Planner) GenerateDeployments() error {
	return p.deployGen.generate(p.config)
}

func (p *Planner) GetConfig() *entity.Config {
	return p.config
}

func (p *Planner) GetState() *DeploymentState {
	return p.plannerService.GetState()
}

func (p *Planner) SetState(state *DeploymentState) {
	p.plannerService.SetState(state)
}

func (p *Planner) LoadStateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var cfg entity.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

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
	for i := range cfg.Certificates {
		state.Certs[cfg.Certificates[i].Name] = &cfg.Certificates[i]
	}
	for i := range cfg.Registries {
		state.Registries[cfg.Registries[i].Name] = &cfg.Registries[i]
	}
	for i := range cfg.ISPs {
		state.ISPs[cfg.ISPs[i].Name] = &cfg.ISPs[i]
	}

	p.plannerService.SetState(state)
	return nil
}

func (p *Planner) SaveStateToFile(path string) error {
	state := p.plannerService.GetState()
	cfg := &entity.Config{
		Services:      make([]entity.BizService, 0, len(state.Services)),
		InfraServices: make([]entity.InfraService, 0, len(state.InfraServices)),
		Servers:       make([]entity.Server, 0, len(state.Servers)),
		Zones:         make([]entity.Zone, 0, len(state.Zones)),
		Domains:       make([]entity.Domain, 0, len(state.Domains)),
		Certificates:  make([]entity.Certificate, 0, len(state.Certs)),
		Registries:    make([]entity.Registry, 0, len(state.Registries)),
		ISPs:          make([]entity.ISP, 0, len(state.ISPs)),
	}

	for _, svc := range state.Services {
		cfg.Services = append(cfg.Services, *svc)
	}
	for _, infra := range state.InfraServices {
		cfg.InfraServices = append(cfg.InfraServices, *infra)
	}
	for _, srv := range state.Servers {
		cfg.Servers = append(cfg.Servers, *srv)
	}
	for _, z := range state.Zones {
		cfg.Zones = append(cfg.Zones, *z)
	}
	for _, d := range state.Domains {
		cfg.Domains = append(cfg.Domains, *d)
	}
	for _, c := range state.Certs {
		cfg.Certificates = append(cfg.Certificates, *c)
	}
	for _, r := range state.Registries {
		cfg.Registries = append(cfg.Registries, *r)
	}
	for _, isp := range state.ISPs {
		cfg.ISPs = append(cfg.ISPs, *isp)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}
