package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (s *DifferService) PlanServers(plan *valueobject.Plan, cfgMap map[string]*entity.Server, zoneMap map[string]*entity.Zone, scope *valueobject.Scope) {
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
	if a.Name != b.Name || a.Zone != b.Zone || a.ISP != b.ISP || a.OS != b.OS {
		return false
	}
	if a.IP.Public != b.IP.Public || a.IP.Private != b.IP.Private {
		return false
	}
	if a.SSH.Host != b.SSH.Host || a.SSH.Port != b.SSH.Port || a.SSH.User != b.SSH.User {
		return false
	}
	if !a.SSH.Password.Equals(&b.SSH.Password) {
		return false
	}
	if a.Environment.APTSource != b.Environment.APTSource {
		return false
	}
	if len(a.Environment.Registries) != len(b.Environment.Registries) {
		return false
	}
	for i, reg := range a.Environment.Registries {
		if i >= len(b.Environment.Registries) || reg != b.Environment.Registries[i] {
			return false
		}
	}
	return true
}

func (s *DifferService) PlanServices(plan *valueobject.Plan, cfgMap map[string]*entity.BizService, serverMap map[string]*entity.Server, scope *valueobject.Scope) {
	for name, state := range s.state.Services {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.Server
			zoneName := ""
			if srv, ok := serverMap[serverName]; ok {
				zoneName = srv.Zone
			}
			if scope.Matches(zoneName, serverName, name, "") {
				plan.AddChange(&valueobject.Change{
					Type:         valueobject.ChangeTypeDelete,
					Entity:       "service",
					Name:         name,
					OldState:     state,
					NewState:     nil,
					Actions:      []string{fmt.Sprintf("delete service %s", name)},
					RemoteExists: true,
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
		if !scope.Matches(zoneName, serverName, name, "") {
			continue
		}

		if state, exists := s.state.Services[name]; exists {
			if scope.ForceDeploy || !ServiceEquals(state, cfg) {
				changeType := valueobject.ChangeTypeUpdate
				if scope.ForceDeploy && ServiceEquals(state, cfg) {
					changeType = valueobject.ChangeTypeCreate
				}
				plan.AddChange(&valueobject.Change{
					Type:         changeType,
					Entity:       "service",
					Name:         name,
					OldState:     state,
					NewState:     cfg,
					Actions:      []string{fmt.Sprintf("deploy service %s", name)},
					RemoteExists: true,
				})
			}
		} else {
			plan.AddChange(&valueobject.Change{
				Type:         valueobject.ChangeTypeCreate,
				Entity:       "service",
				Name:         name,
				OldState:     nil,
				NewState:     cfg,
				Actions:      []string{fmt.Sprintf("create service %s", name)},
				RemoteExists: false,
			})
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
		if bv, ok := b.Env[k]; !ok || !v.Equals(&bv) {
			return false
		}
	}
	if len(a.Secrets) != len(b.Secrets) {
		return false
	}
	for i, sec := range a.Secrets {
		if i >= len(b.Secrets) || sec != b.Secrets[i] {
			return false
		}
	}
	if !healthcheckEqual(a.Healthcheck, b.Healthcheck) {
		return false
	}
	if a.Resources != b.Resources {
		return false
	}
	if len(a.Volumes) != len(b.Volumes) {
		return false
	}
	for i, vol := range a.Volumes {
		if i >= len(b.Volumes) || vol != b.Volumes[i] {
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
	if a.Internal != b.Internal {
		return false
	}
	if len(a.Networks) != len(b.Networks) {
		return false
	}
	for i, net := range a.Networks {
		if i >= len(b.Networks) || net != b.Networks[i] {
			return false
		}
	}
	return true
}

func healthcheckEqual(a, b *entity.ServiceHealthcheck) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Path == b.Path && a.Interval == b.Interval && a.Timeout == b.Timeout
}

func (s *DifferService) PlanInfraServices(plan *valueobject.Plan, cfgMap map[string]*entity.InfraService, serverMap map[string]*entity.Server, scope *valueobject.Scope) {
	for name, state := range s.state.InfraServices {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.Server
			zoneName := ""
			if srv, ok := serverMap[serverName]; ok {
				zoneName = srv.Zone
			}
			if scope.MatchesInfra(zoneName, serverName, name) {
				plan.AddChange(&valueobject.Change{
					Type:         valueobject.ChangeTypeDelete,
					Entity:       "infra_service",
					Name:         name,
					OldState:     state,
					NewState:     nil,
					Actions:      []string{fmt.Sprintf("delete infra service %s", name)},
					RemoteExists: true,
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
		if !scope.MatchesInfra(zoneName, serverName, name) {
			continue
		}

		if state, exists := s.state.InfraServices[name]; exists {
			if scope.ForceDeploy || !InfraServiceEquals(state, cfg) {
				changeType := valueobject.ChangeTypeUpdate
				if scope.ForceDeploy && InfraServiceEquals(state, cfg) {
					changeType = valueobject.ChangeTypeCreate
				}
				plan.AddChange(&valueobject.Change{
					Type:         changeType,
					Entity:       "infra_service",
					Name:         name,
					OldState:     state,
					NewState:     cfg,
					Actions:      []string{fmt.Sprintf("deploy infra service %s", name)},
					RemoteExists: true,
				})
			}
		} else {
			plan.AddChange(&valueobject.Change{
				Type:         valueobject.ChangeTypeCreate,
				Entity:       "infra_service",
				Name:         name,
				OldState:     nil,
				NewState:     cfg,
				Actions:      []string{fmt.Sprintf("create infra service %s", name)},
				RemoteExists: false,
			})
		}
	}
}

func InfraServiceEquals(a, b *entity.InfraService) bool {
	if a.Name != b.Name || a.Server != b.Server || a.Image != b.Image || a.Type != b.Type {
		return false
	}
	if a.GatewayLogLevel != b.GatewayLogLevel {
		return false
	}
	if !gatewayPortsEqual(a.GatewayPorts, b.GatewayPorts) {
		return false
	}
	if !gatewayConfigEqual(a.GatewayConfig, b.GatewayConfig) {
		return false
	}
	if !gatewaySSLConfigEqual(a.GatewaySSL, b.GatewaySSL) {
		return false
	}
	if !gatewayWAFConfigEqual(a.GatewayWAF, b.GatewayWAF) {
		return false
	}
	if !sslConfigEqual(a.SSLConfig, b.SSLConfig) {
		return false
	}
	return true
}

func gatewayPortsEqual(a, b *entity.GatewayPorts) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.HTTP == b.HTTP && a.HTTPS == b.HTTPS
}

func gatewayConfigEqual(a, b *entity.GatewayConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Source == b.Source && a.Sync == b.Sync
}

func gatewaySSLConfigEqual(a, b *entity.GatewaySSLConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Mode == b.Mode && a.Endpoint == b.Endpoint
}

func gatewayWAFConfigEqual(a, b *entity.GatewayWAFConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Enabled != b.Enabled {
		return false
	}
	if len(a.Whitelist) != len(b.Whitelist) {
		return false
	}
	for i, w := range a.Whitelist {
		if i >= len(b.Whitelist) || w != b.Whitelist[i] {
			return false
		}
	}
	return true
}

func sslConfigEqual(a, b *entity.SSLConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Ports.API != b.Ports.API {
		return false
	}
	if a.Config == nil && b.Config == nil {
		return true
	}
	if a.Config == nil || b.Config == nil {
		return false
	}
	return a.Config.Source == b.Config.Source && a.Config.Sync == b.Config.Sync
}
