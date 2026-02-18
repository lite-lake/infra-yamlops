package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type EntityComparer[T any] interface {
	Equals(a, b *T) bool
}

type PlannerService struct {
	state *repository.DeploymentState
}

func NewPlannerService(state *repository.DeploymentState) *PlannerService {
	if state == nil {
		state = &repository.DeploymentState{
			Services:   make(map[string]*entity.BizService),
			Gateways:   make(map[string]*entity.Gateway),
			Servers:    make(map[string]*entity.Server),
			Zones:      make(map[string]*entity.Zone),
			Domains:    make(map[string]*entity.Domain),
			Records:    make(map[string]*entity.DNSRecord),
			Certs:      make(map[string]*entity.Certificate),
			Registries: make(map[string]*entity.Registry),
			ISPs:       make(map[string]*entity.ISP),
		}
	}
	return &PlannerService{state: state}
}

func (s *PlannerService) GetState() *repository.DeploymentState {
	return s.state
}

func (s *PlannerService) SetState(state *repository.DeploymentState) {
	s.state = state
}

func (s *PlannerService) PlanISPs(plan *valueobject.Plan, cfgMap map[string]*entity.ISP, scope *valueobject.Scope) {
	for name, state := range s.state.ISPs {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "isp",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete isp %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := s.state.ISPs[name]; exists {
			if !ISPEquals(state, cfg) {
				if scope.Matches("", "", "", "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "isp",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update isp %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "isp",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create isp %s", name)},
				})
			}
		}
	}
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

func (s *PlannerService) PlanZones(plan *valueobject.Plan, cfgMap map[string]*entity.Zone, scope *valueobject.Scope) {
	for name, state := range s.state.Zones {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches(name, "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "zone",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete zone %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := s.state.Zones[name]; exists {
			if !ZoneEquals(state, cfg) {
				if scope.Matches(name, "", "", "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "zone",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update zone %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(name, "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "zone",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create zone %s", name)},
				})
			}
		}
	}
}

func ZoneEquals(a, b *entity.Zone) bool {
	return a.Name == b.Name && a.ISP == b.ISP && a.Region == b.Region
}

func (s *PlannerService) PlanDomains(plan *valueobject.Plan, cfgMap map[string]*entity.Domain, scope *valueobject.Scope) {
	for name, state := range s.state.Domains {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", name) {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "domain",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete domain %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := s.state.Domains[name]; exists {
			if !DomainEquals(state, cfg) {
				if scope.Matches("", "", "", name) {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "domain",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update domain %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", name) {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "domain",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create domain %s", name)},
				})
			}
		}
	}
}

func DomainEquals(a, b *entity.Domain) bool {
	return a.Name == b.Name && a.ISP == b.ISP && a.Parent == b.Parent
}
