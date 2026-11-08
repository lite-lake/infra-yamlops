package service

import (
	"fmt"
	"reflect"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func stringSlicesEqualUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	mapA := make(map[string]struct{}, len(a))
	for _, s := range a {
		mapA[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := mapA[s]; !ok {
			return false
		}
	}
	return true
}

func comparableSlicesEqualUnordered[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	mapA := make(map[T]struct{}, len(a))
	for _, item := range a {
		mapA[item] = struct{}{}
	}
	for _, item := range b {
		if _, ok := mapA[item]; !ok {
			return false
		}
	}
	return true
}

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
			if scope.Matches(zoneName, name, "", "", "", "") {
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
				if scope.Matches(zoneName, name, "", "", "", "") {
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
			if scope.Matches(zoneName, name, "", "", "", "") {
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
	if !stringSlicesEqualUnordered(a.Environment.Registries, b.Environment.Registries) {
		return false
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
				isForcedNoChange := scope.ForceDeploy() && equals(state, cfg)
				plan.AddChange(valueobject.NewChangeFull(
					changeType,
					entityType,
					name,
					state,
					cfg,
					[]string{fmt.Sprintf("deploy %s %s", entityType, name)},
					true,
				).WithForcedNoChange(isForcedNoChange))
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
			return scope.Matches(zoneName, serverName, serviceName, "", "", "")
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
			return scope.Matches(zoneName, serverName, serviceName, "", "", "")
		},
		"service",
		ServiceEquals,
	)
}

func ServiceEquals(a, b *entity.BizService) bool {
	if a.Name != b.Name || a.Server != b.Server || a.Image != b.Image {
		return false
	}
	if !comparableSlicesEqualUnordered(a.Ports, b.Ports) {
		return false
	}
	if len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if bv, ok := b.Env[k]; !ok || !v.Equals(&bv) {
			return false
		}
	}
	if !comparableSlicesEqualUnordered(a.Secrets, b.Secrets) {
		return false
	}
	if !healthcheckEqual(a.Healthcheck, b.Healthcheck) {
		return false
	}
	if a.Resources != b.Resources {
		return false
	}
	if !comparableSlicesEqualUnordered(a.Volumes, b.Volumes) {
		return false
	}
	if !gatewayRoutesEqual(a.Gateways, b.Gateways) {
		return false
	}
	if a.Internal != b.Internal {
		return false
	}
	if !stringSlicesEqualUnordered(a.Networks, b.Networks) {
		return false
	}
	return true
}

func gatewayRoutesEqual(a, b []entity.ServiceGatewayRoute) bool {
	return gatewaySlicesEqual(a, b, gatewayRouteEqual)
}

func gatewaySlicesEqual[T any](a, b []T, eq func(x, y T) bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !eq(a[i], b[i]) {
			return false
		}
	}
	return true
}

func gatewayRouteEqual(x, y entity.ServiceGatewayRoute) bool {
	return x.Hostname == y.Hostname &&
		x.ContainerPort == y.ContainerPort &&
		x.Path == y.Path &&
		x.HTTP == y.HTTP &&
		x.HTTPS == y.HTTPS &&
		ptrBoolEqual(x.GzipEnabled, y.GzipEnabled) &&
		x.OverrideHost == y.OverrideHost &&
		ptrBoolEqual(x.StripProxyHeaders, y.StripProxyHeaders) &&
		ptrBoolEqual(x.RobotsDenyAll, y.RobotsDenyAll) &&
		stringSlicesEqual(x.AllowedIPs, y.AllowedIPs) &&
		routeWAFEqual(x.WAF, y.WAF) &&
		x.BlacklistMode == y.BlacklistMode &&
		// 逐条目 http→https 301 + 规范域收敛 + HSTS per-host（内部总体方案 W3-2 / D8）
		ptrBoolEqual(x.RedirectHTTPS, y.RedirectHTTPS) &&
		x.RedirectHost == y.RedirectHost &&
		ptrStringEqual(x.HSTS, y.HSTS) &&
		// L3 per-host WSS 上游证书校验跳过（内部PRD）
		ptrBoolEqual(x.WebSocketInsecureSkip, y.WebSocketInsecureSkip) &&
		routeAuthEqual(x.Auth, y.Auth)
}

func routeWAFEqual(a, b *entity.RouteWAFConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Mode == b.Mode
}

// routeAuthEqual 密码门段相等性（内部PRD；M3 防爆破参数 内部PRD）。
// Password 按 SecretRef 引用相等比较（不解明文，state 侧不携带明文密钥，
// 与 gatewayTrackerShipperConfigEqual 的 SharedSecret 同口径）。
// 注：此前 gatewayRouteEqual 漏比较整个 Auth 段（v1.0.23 引入时的缺口），
// 改动 auth 配置不会产生 entity 级 diff——随本次新字段接线一并修复。
func routeAuthEqual(a, b *entity.RouteAuthConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if (a.Password == nil) != (b.Password == nil) {
		return false
	}
	if a.Password != nil && !a.Password.Equals(b.Password) {
		return false
	}
	return a.TTL == b.TTL &&
		stringSlicesEqual(a.PublicPaths, b.PublicPaths) &&
		a.MaxFailures == b.MaxFailures &&
		a.LockDuration == b.LockDuration
}

func ptrBoolEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ptrStringEqual *string 指针相等（nil 视作"未设置"，两 nil 相等；单边 nil 不等）
func ptrStringEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
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
			return scope.Matches(zoneName, serverName, "", serviceName, "", "")
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
			return scope.Matches(zoneName, serverName, "", serviceName, "", "")
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
	if a.Resources != b.Resources {
		return false
	}
	if !gatewayPortsEqual(a.GatewayPorts, b.GatewayPorts) {
		return false
	}
	if !gatewaySSLConfigEqual(a.GatewaySSL, b.GatewaySSL) {
		return false
	}
	if !gatewayWAFConfigEqual(a.GatewayWAF, b.GatewayWAF) {
		return false
	}
	if !gatewayBlacklistConfigEqual(a.GatewayBlacklist, b.GatewayBlacklist) {
		return false
	}
	// trusted_proxies 顺序不敏感（语义为网段集合，与 waf/blacklist whitelist 一致）
	if !stringSlicesEqualUnordered(a.GatewayTrustedProxies, b.GatewayTrustedProxies) {
		return false
	}
	// H1 路径规范化强制逃生口 + L3 全局 WSS 跳过校验（内部PRD）
	if !ptrBoolEqual(a.GatewayStrictPathCheck, b.GatewayStrictPathCheck) {
		return false
	}
	if !ptrBoolEqual(a.GatewayWebSocketInsecureSkip, b.GatewayWebSocketInsecureSkip) {
		return false
	}
	if !gatewayTrackerShipperConfigEqual(a.GatewayTrackerShipper, b.GatewayTrackerShipper) {
		return false
	}
	if !tunnelServerConfigEqual(a.TunnelServerConfig, b.TunnelServerConfig) {
		return false
	}
	if !tunnelClientConfigEqual(a.TunnelClientConfig, b.TunnelClientConfig) {
		return false
	}
	if !easyTierConfigEqual(a.EasyTierConfig, b.EasyTierConfig) {
		return false
	}
	if !uptimeKumaConfigEqual(a.UptimeKumaConfig, b.UptimeKumaConfig) {
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
		if x.DefaultMode != y.DefaultMode {
			return false
		}
		if !stringSlicesEqualUnordered(x.Whitelist, y.Whitelist) {
			return false
		}
		if x.CRSProxyURL != y.CRSProxyURL {
			return false
		}
		if x.CRSMirrorURL != y.CRSMirrorURL {
			return false
		}
		if x.CRSFailurePolicy != y.CRSFailurePolicy {
			return false
		}
		// H2 CRS pin / 明文镜像放行开关（内部PRD）
		if x.CRSPinnedSHA256 != y.CRSPinnedSHA256 {
			return false
		}
		if !ptrBoolEqual(x.CRSMirrorAllowInsecureHTTP, y.CRSMirrorAllowInsecureHTTP) {
			return false
		}
		return true
	})
}

func gatewayBlacklistConfigEqual(a, b *entity.GatewayBlacklistConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.GatewayBlacklistConfig) bool {
		if x.Enabled != y.Enabled {
			return false
		}
		if x.DefaultMode != y.DefaultMode {
			return false
		}
		if !stringSlicesEqualUnordered(x.Whitelist, y.Whitelist) {
			return false
		}
		if x.SourceURL != y.SourceURL {
			return false
		}
		// L6 明文 http 源放行开关 / 增长上限基数（内部PRD）
		if !ptrBoolEqual(x.AllowHTTPSource, y.AllowHTTPSource) {
			return false
		}
		if x.MaxEntries != y.MaxEntries {
			return false
		}
		return true
	})
}

// gatewayTrackerShipperConfigEqual tracker_shipper 段相等性（内部PRD §7.1）。
// SharedSecret 按 SecretRef 引用相等比较（不解明文，state 侧不携带明文密钥）。
func gatewayTrackerShipperConfigEqual(a, b *entity.GatewayTrackerShipperConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.GatewayTrackerShipperConfig) bool {
		if x.Enabled != y.Enabled || x.Endpoint != y.Endpoint || x.NodeName != y.NodeName {
			return false
		}
		if x.BufferSize != y.BufferSize || x.BatchSize != y.BatchSize || x.FlushInterval != y.FlushInterval {
			return false
		}
		if x.FailureSampleLogRatio != y.FailureSampleLogRatio {
			return false
		}
		return x.SharedSecret.Equals(&y.SharedSecret)
	})
}

func tunnelServerConfigEqual(a, b *entity.TunnelServerConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.TunnelServerConfig) bool {
		if x.Ports != y.Ports {
			return false
		}
		if !x.AuthToken.Equals(&y.AuthToken) {
			return false
		}
		if len(x.TunnelRoutes) != len(y.TunnelRoutes) {
			return false
		}
		for i := range x.TunnelRoutes {
			if x.TunnelRoutes[i].Hostname != y.TunnelRoutes[i].Hostname {
				return false
			}
		}
		return true
	})
}

func tunnelClientConfigEqual(a, b *entity.TunnelClientConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.TunnelClientConfig) bool {
		if x.ServerAddr != y.ServerAddr {
			return false
		}
		if !x.AuthToken.Equals(&y.AuthToken) {
			return false
		}
		if len(x.Proxies) != len(y.Proxies) {
			return false
		}
		for i := range x.Proxies {
			px, py := &x.Proxies[i], &y.Proxies[i]
			if px.Name != py.Name || px.Type != py.Type || px.LocalAddr != py.LocalAddr || px.RemotePort != py.RemotePort {
				return false
			}
			if !stringSlicesEqualUnordered(px.CustomDomains, py.CustomDomains) {
				return false
			}
		}
		return true
	})
}

func easyTierConfigEqual(a, b *entity.EasyTierConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.EasyTierConfig) bool {
		if x.Role != y.Role || x.VirtualIP != y.VirtualIP || x.NetworkName != y.NetworkName || x.Hostname != y.Hostname {
			return false
		}
		if !x.NetworkSecret.Equals(&y.NetworkSecret) {
			return false
		}
		// 顺序敏感：listeners/peers 顺序变化会改变生成的 toml 内容，应触发重建
		if !stringSlicesEqualUnordered(x.Listeners, y.Listeners) {
			return false
		}
		if !stringSlicesEqualUnordered(x.Peers, y.Peers) {
			return false
		}
		fx, fy := x.Flags, y.Flags
		if fx.LatencyFirst != fy.LatencyFirst ||
			fx.PrivateMode != fy.PrivateMode ||
			fx.RelayNetworkWhitelist != fy.RelayNetworkWhitelist ||
			fx.RelayAllPeerRPC != fy.RelayAllPeerRPC ||
			fx.NoListener != fy.NoListener {
			return false
		}
		return true
	})
}

// uptimeKumaConfigEqual 比较 uptime-kuma 配置中影响 compose/路由/目标文件重建的字段。
// monitors 切片按索引顺序比较：monitors 顺序影响生成的 kuma.targets.yaml 内容，
// 且只改监控目标时需要触发部署以把新的 targets.yaml 同步到服务器（agent 文件监听自愈的前提）。
func uptimeKumaConfigEqual(a, b *entity.UptimeKumaConfig) bool {
	return ptrEqual(a, b, func(x, y *entity.UptimeKumaConfig) bool {
		if x.Admin.Username != y.Admin.Username || !x.Admin.Password.Equals(&y.Admin.Password) {
			return false
		}
		if x.DB.Type != y.DB.Type ||
			x.DB.Hostname != y.DB.Hostname ||
			x.DB.Port != y.DB.Port ||
			x.DB.Name != y.DB.Name ||
			x.DB.Username != y.DB.Username ||
			!x.DB.Password.Equals(&y.DB.Password) {
			return false
		}
		if len(x.Gateways) != len(y.Gateways) {
			return false
		}
		for i := range x.Gateways {
			gx, gy := &x.Gateways[i], &y.Gateways[i]
			if gx.Hostname != gy.Hostname || gx.HTTPS != gy.HTTPS || gx.ContainerPort != gy.ContainerPort {
				return false
			}
		}
		if x.Sync.Image != y.Sync.Image || x.Sync.ReconcileInterval != y.Sync.ReconcileInterval ||
			x.Sync.ReloginInterval != y.Sync.ReloginInterval {
			return false
		}
		if x.Notification.Enabled != y.Notification.Enabled || x.Notification.URL != y.Notification.URL {
			return false
		}
		if !uptimeKumaMonitorConfigsEqual(x.Monitors, y.Monitors) {
			return false
		}
		return true
	})
}

// uptimeKumaMonitorConfigsEqual 比较 monitors 切片，顺序敏感（按索引逐项比较，同 tunnel_routes 处理方式）：
// monitors 顺序影响生成的 kuma.targets.yaml 内容，顺序变化应触发重建。
func uptimeKumaMonitorConfigsEqual(a, b []entity.UptimeKumaMonitorConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !uptimeKumaMonitorConfigEqual(&a[i], &b[i]) {
			return false
		}
	}
	return true
}

// uptimeKumaMonitorConfigEqual 比较单个 monitor 的全部字段；accepted_statuscodes/tags 切片内逐元素比较，
// Extra（透传字段 map）做深比较——Extra 变化必须触发 targets 重建。
func uptimeKumaMonitorConfigEqual(x, y *entity.UptimeKumaMonitorConfig) bool {
	return x.Name == y.Name &&
		x.Type == y.Type &&
		x.URL == y.URL &&
		x.Interval == y.Interval &&
		stringSlicesEqual(x.AcceptedStatuscodes, y.AcceptedStatuscodes) &&
		x.MaxRedirects == y.MaxRedirects &&
		x.ExpiryNotification == y.ExpiryNotification &&
		x.DomainExpiryNotification == y.DomainExpiryNotification &&
		x.Keyword == y.Keyword &&
		x.Hostname == y.Hostname &&
		x.Port == y.Port &&
		stringSlicesEqual(x.Tags, y.Tags) &&
		reflect.DeepEqual(x.Extra, y.Extra)
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
