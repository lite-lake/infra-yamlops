package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type serviceEntity interface {
	GetServer() string
}

type matchScopeFunc func(zoneName, serverName, serviceName string) bool

func (s *DifferService) PlanServers(plan *valueobject.Plan, cfgMap map[string]*entity.Server, zoneMap map[string]*entity.Zone, scope *valueobject.Scope) {
	for name, state := range s.state.Servers {
		if _, exists := cfgMap[name]; !exists {
			zoneName := ""
			if state.Zone != "" {
				zoneName = state.Zone
			}
			if scope.Matches(zoneName, name, "", "") {
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeDelete,
					"server",
					name,
					state,
					nil,
					[]string{fmt.Sprintf("delete server %s", name)},
					false,
				))
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
					plan.AddChange(valueobject.NewChangeFull(
						valueobject.ChangeTypeUpdate,
						"server",
						name,
						state,
						cfg,
						[]string{fmt.Sprintf("update server %s", name)},
						false,
					))
				}
			}
		} else {
			if scope.Matches(zoneName, name, "", "") {
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeCreate,
					"server",
					name,
					nil,
					cfg,
					[]string{fmt.Sprintf("create server %s", name)},
					false,
				))
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
	if len(a.Networks) != len(b.Networks) {
		return false
	}
	// 使用 map 进行顺序不敏感比较，比较所有字段
	netMapA := make(map[string]entity.ServerNetwork)
	for _, n := range a.Networks {
		netMapA[n.Name] = n
	}
	for _, n := range b.Networks {
		netA, ok := netMapA[n.Name]
		if !ok {
			return false
		}
		if netA.Type != n.Type || netA.Driver != n.Driver {
			return false
		}
	}
	return true
}

func planServiceDeletions[T serviceEntity](
	plan *valueobject.Plan,
	stateMap map[string]T,
	cfgMap map[string]T,
	serverMap map[string]*entity.Server,
	scope *valueobject.Scope,
	matchScope matchScopeFunc,
	entityType string,
) {
	for name, state := range stateMap {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.GetServer()
			zoneName := ""
			if srv, ok := serverMap[serverName]; ok {
				zoneName = srv.Zone
			}
			if matchScope(zoneName, serverName, name) {
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeDelete,
					entityType,
					name,
					state,
					nil,
					[]string{fmt.Sprintf("delete %s %s", entityType, name)},
					true,
				))
			}
		}
	}
}

func planServiceUpdatesAndCreates[T serviceEntity](
	plan *valueobject.Plan,
	stateMap map[string]T,
	cfgMap map[string]T,
	serverMap map[string]*entity.Server,
	scope *valueobject.Scope,
	matchScope matchScopeFunc,
	entityType string,
	equals func(a, b T) bool,
) {
	for name, cfg := range cfgMap {
		serverName := cfg.GetServer()
		zoneName := ""
		if srv, ok := serverMap[serverName]; ok {
			zoneName = srv.Zone
		}
		if !matchScope(zoneName, serverName, name) {
			continue
		}

		if state, exists := stateMap[name]; exists {
			if scope.ForceDeploy() || !equals(state, cfg) {
				changeType := valueobject.ChangeTypeUpdate
				if scope.ForceDeploy() && equals(state, cfg) {
					changeType = valueobject.ChangeTypeCreate
				}
				plan.AddChange(valueobject.NewChangeFull(
					changeType,
					entityType,
					name,
					state,
					cfg,
					[]string{fmt.Sprintf("deploy %s %s", entityType, name)},
					true,
				))
			}
		} else {
			plan.AddChange(valueobject.NewChangeFull(
				valueobject.ChangeTypeCreate,
				entityType,
				name,
				nil,
				cfg,
				[]string{fmt.Sprintf("create %s %s", entityType, name)},
				false,
			))
		}
	}
}

func (s *DifferService) PlanServices(plan *valueobject.Plan, cfgMap map[string]*entity.BizService, serverMap map[string]*entity.Server, scope *valueobject.Scope) {
	planServiceDeletions(
		plan,
		s.state.Services,
		cfgMap,
		serverMap,
		scope,
		func(zoneName, serverName, serviceName string) bool {
			return scope.Matches(zoneName, serverName, serviceName, "")
		},
		"service",
	)
	planServiceUpdatesAndCreates(
		plan,
		s.state.Services,
		cfgMap,
		serverMap,
		scope,
		func(zoneName, serverName, serviceName string) bool {
			return scope.Matches(zoneName, serverName, serviceName, "")
		},
		"service",
		ServiceEquals,
	)
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
	return ptrEqual(a, b, func(x, y *entity.ServiceHealthcheck) bool {
		return x.Path == y.Path && x.Interval == y.Interval && x.Timeout == y.Timeout
	})
}

func (s *DifferService) PlanInfraServices(plan *valueobject.Plan, cfgMap map[string]*entity.InfraService, serverMap map[string]*entity.Server, scope *valueobject.Scope) {
	planServiceDeletions(
		plan,
		s.state.InfraServices,
		cfgMap,
		serverMap,
		scope,
		func(zoneName, serverName, serviceName string) bool {
			return scope.MatchesInfra(zoneName, serverName, serviceName)
		},
		"infra_service",
	)
	planServiceUpdatesAndCreates(
		plan,
		s.state.InfraServices,
		cfgMap,
		serverMap,
		scope,
		func(zoneName, serverName, serviceName string) bool {
			return scope.MatchesInfra(zoneName, serverName, serviceName)
		},
		"infra_service",
		InfraServiceEquals,
	)
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
	// Networks: order-insensitive comparison
	if len(a.Networks) != len(b.Networks) {
		return false
	}
	netMap := make(map[string]struct{})
	for _, net := range a.Networks {
		netMap[net] = struct{}{}
	}
	for _, net := range b.Networks {
		if _, ok := netMap[net]; !ok {
			return false
		}
	}
	return true
}

func gatewayPortsEqual(a, b *entity.GatewayPorts) bool {
	return ptrEqual(a, b, func(x, y *entity.GatewayPorts) bool {
		return x.HTTP == y.HTTP && x.HTTPS == y.HTTPS
	})
}

func gatewayConfigEqual(a, b *entity.GatewayConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.GatewayConfig) bool {
		return x.Source == y.Source && x.Sync == y.Sync
	})
}

func gatewaySSLConfigEqual(a, b *entity.GatewaySSLConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.GatewaySSLConfig) bool {
		return x.Mode == y.Mode && x.Endpoint == y.Endpoint
	})
}

func gatewayWAFConfigEqual(a, b *entity.GatewayWAFConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.GatewayWAFConfig) bool {
		if x.Enabled != y.Enabled {
			return false
		}
		if len(x.Whitelist) != len(y.Whitelist) {
			return false
		}
		for i, w := range x.Whitelist {
			if i >= len(y.Whitelist) || w != y.Whitelist[i] {
				return false
			}
		}
		return true
	})
}

func sslConfigEqual(a, b *entity.SSLConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.SSLConfig) bool {
		if x.Ports.API != y.Ports.API {
			return false
		}
		if x.Config == nil && y.Config == nil {
			return true
		}
		if x.Config == nil || y.Config == nil {
			return false
		}
		return x.Config.Source == y.Config.Source && x.Config.Sync == y.Config.Sync
	})
}

func ptrEqual[T any](a, b *T, eq func(a, b *T) bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return eq(a, b)
}
