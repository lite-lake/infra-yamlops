package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (s *PlannerService) PlanServers(plan *valueobject.Plan, cfgMap map[string]*entity.Server, zoneMap map[string]*entity.Zone, scope *valueobject.Scope) {
	for name, state := range s.state.Servers {
		if _, exists := cfgMap[name]; !exists {
			zoneName := ""
			if state.Zone != "" {
				zoneName = state.Zone
			}
			if scope.Matches(zoneName, name, "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "server",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete server %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		zoneName := ""
		if cfg.Zone != "" {
			if z, ok := zoneMap[cfg.Zone]; ok {
				zoneName = z.Name
			}
		}
		if state, exists := s.state.Servers[name]; exists {
			if !ServerEquals(state, cfg) {
				if scope.Matches(zoneName, name, "", "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "server",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update server %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(zoneName, name, "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "server",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create server %s", name)},
				})
			}
		}
	}
}

func ServerEquals(a, b *entity.Server) bool {
	return a.Name == b.Name && a.Zone == b.Zone && a.ISP == b.ISP &&
		a.IP.Public == b.IP.Public && a.IP.Private == b.IP.Private
}

func (s *PlannerService) PlanGateways(plan *valueobject.Plan, cfgMap map[string]*entity.Gateway, serverMap map[string]*entity.Server, scope *valueobject.Scope) {
	for name, state := range s.state.Gateways {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.Server
			zoneName := ""
			if srv, ok := serverMap[serverName]; ok {
				zoneName = srv.Zone
			}
			if scope.Matches(zoneName, serverName, "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "gateway",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete gateway %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		serverName := cfg.Server
		zoneName := ""
		if srv, ok := serverMap[serverName]; ok {
			zoneName = srv.Zone
		}
		if state, exists := s.state.Gateways[name]; exists {
			if !GatewayEquals(state, cfg) {
				if scope.Matches(zoneName, serverName, "", "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "gateway",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update gateway %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(zoneName, serverName, "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "gateway",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create gateway %s", name)},
				})
			}
		}
	}
}

func GatewayEquals(a, b *entity.Gateway) bool {
	return a.Name == b.Name && a.Zone == b.Zone && a.Server == b.Server &&
		a.Image == b.Image && a.Ports.HTTP == b.Ports.HTTP && a.Ports.HTTPS == b.Ports.HTTPS
}

func (s *PlannerService) PlanServices(plan *valueobject.Plan, cfgMap map[string]*entity.BizService, serverMap map[string]*entity.Server, scope *valueobject.Scope) {
	for name, state := range s.state.Services {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.Server
			zoneName := ""
			if srv, ok := serverMap[serverName]; ok {
				zoneName = srv.Zone
			}
			if scope.Matches(zoneName, serverName, name, "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "service",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete service %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		serverName := cfg.Server
		zoneName := ""
		if srv, ok := serverMap[serverName]; ok {
			zoneName = srv.Zone
		}
		if state, exists := s.state.Services[name]; exists {
			if !ServiceEquals(state, cfg) {
				if scope.Matches(zoneName, serverName, name, "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "service",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update service %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(zoneName, serverName, name, "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "service",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create service %s", name)},
				})
			}
		}
	}
}

func ServiceEquals(a, b *entity.BizService) bool {
	if a.Name != b.Name || a.Server != b.Server || a.Image != b.Image {
		return false
	}
	if len(a.Ports) != len(b.Ports) {
		return false
	}
	for i, port := range a.Ports {
		if i >= len(b.Ports) || port != b.Ports[i] {
			return false
		}
	}
	if len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if bv, ok := b.Env[k]; !ok || bv != v {
			return false
		}
	}
	if len(a.Gateways) != len(b.Gateways) {
		return false
	}
	for i, gw := range a.Gateways {
		if i >= len(b.Gateways) || gw != b.Gateways[i] {
			return false
		}
	}
	return true
}
