package service

import (
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type DifferService struct {
	state *repository.DeploymentState
}

func NewDifferService(state *repository.DeploymentState) *DifferService {
	if state == nil {
		state = &repository.DeploymentState{
			Services:      make(map[string]*entity.BizService),
			InfraServices: make(map[string]*entity.InfraService),
			Servers:       make(map[string]*entity.Server),
			Zones:         make(map[string]*entity.Zone),
			Domains:       make(map[string]*entity.Domain),
			Records:       make(map[string]*entity.DNSRecord),
			ISPs:          make(map[string]*entity.ISP),
		}
	}
	return &DifferService{state: state}
}

func (s *DifferService) GetState() *repository.DeploymentState {
	return s.state
}

func (s *DifferService) SetState(state *repository.DeploymentState) {
	s.state = state
}

func (s *DifferService) PlanISPs(plan *valueobject.Plan, cfgMap map[string]*entity.ISP, scope *valueobject.Scope) {
	planSimpleEntity(plan, cfgMap, s.state.ISPs, ISPEquals, "isp",
		func(_ string) bool { return scope.Matches("", "", "", "") })
}

func ISPEquals(a, b *entity.ISP) bool {
	if a.Name != b.Name {
		return false
	}
	if len(a.Services) != len(b.Services) {
		return false
	}
	for i, s := range a.Services {
		if i >= len(b.Services) || s != b.Services[i] {
			return false
		}
	}
	return true
}

func (s *DifferService) PlanZones(plan *valueobject.Plan, cfgMap map[string]*entity.Zone, scope *valueobject.Scope) {
	planSimpleEntity(plan, cfgMap, s.state.Zones, ZoneEquals, "zone",
		func(name string) bool { return scope.Matches(name, "", "", "") })
}

func ZoneEquals(a, b *entity.Zone) bool {
	return a.Name == b.Name && a.ISP == b.ISP && a.Region == b.Region
}

func (s *DifferService) PlanDomains(plan *valueobject.Plan, cfgMap map[string]*entity.Domain, scope *valueobject.Scope) {
	planSimpleEntity(plan, cfgMap, s.state.Domains, DomainEquals, "domain",
		func(_ string) bool { return scope.Matches("", "", "", "") })
}

func DomainEquals(a, b *entity.Domain) bool {
	return a.Name == b.Name && a.ISP == b.ISP && a.Parent == b.Parent
}
