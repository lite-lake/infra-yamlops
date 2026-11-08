package generator

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/application/generator/compose"
	"github.com/lite-lake/infra-yamlops/internal/application/generator/gate"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

type gatewayRouteResult struct {
	hosts         []gate.HostRoute
	gatewayConfig *gate.GatewayConfig
	httpPort      int
	httpsPort     int
}

func (g *Generator) generateGatewayConfigs(config *entity.Config) error {
	gatewayServers := make(map[string][]*entity.InfraService)
	for i := range config.InfraServices {
		infra := &config.InfraServices[i]
		if infra.Type == entity.InfraServiceTypeGateway {
			gatewayServers[infra.Server] = append(gatewayServers[infra.Server], infra)
		}
	}

	for serverName, gateways := range gatewayServers {
		serverDir := filepath.Join(g.outputDir, g.env, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverDir, err)
		}

		for _, gw := range gateways {
			if err := g.generateGatewayConfig(serverDir, gw, config); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Generator) generateGatewayConfigsWithScope(config *entity.Config, scope *valueobject.Scope) error {
	gatewayServers := make(map[string][]*entity.InfraService)
	for i := range config.InfraServices {
		infra := &config.InfraServices[i]
		if infra.Type == entity.InfraServiceTypeGateway && scope.ShouldGenerateInfraService(infra.Name, infra.Server) {
			gatewayServers[infra.Server] = append(gatewayServers[infra.Server], infra)
		}
	}

	for serverName, gateways := range gatewayServers {
		serverDir := filepath.Join(g.outputDir, g.env, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverDir, err)
		}

		for _, gw := range gateways {
			if err := g.generateGatewayConfig(serverDir, gw, config); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Generator) buildGatewayRoutes(gw *entity.InfraService, config *entity.Config) (*gatewayRouteResult, error) {
	hosts, err := g.collectHostRoutes(gw, config)
	if err != nil {
		return nil, err
	}
	gatewayConfig := g.buildGatewayConfig(gw)

	// tracker_shipper（内部PRD §7.1）：{secret: name} 引用在此解析为明文，
	// 解析失败直接报错不产出半成品 gate.yaml；未配置（nil）时保持 nil，
	// 产物不生成 tracker_shipper 键（存量输出逐字节不变）。
	trackerShipper, err := g.buildTrackerShipperConfig(gw, config.GetSecretsMap())
	if err != nil {
		return nil, err
	}
	gatewayConfig.TrackerShipper = trackerShipper

	return &gatewayRouteResult{
		hosts:         hosts,
		gatewayConfig: gatewayConfig,
		httpPort:      gatewayConfig.Port,
		httpsPort:     constants.DefaultHTTPSPort,
	}, nil
}

func (g *Generator) collectHostRoutes(gw *entity.InfraService, config *entity.Config) ([]gate.HostRoute, error) {
	secrets := config.GetSecretsMap()
	var hosts []gate.HostRoute
	for _, svc := range config.Services {
		if svc.Server != gw.Server {
			continue
		}
		for _, route := range svc.Gateways {
			if !route.HasGateway() {
				continue
			}
			hr, err := g.buildHostRoute(&svc, &route, gw, secrets)
			if err != nil {
				return nil, fmt.Errorf("service %s gateway %s: %w", svc.Name, route.Hostname, err)
			}
			hosts = append(hosts, hr)
		}
	}

	hosts = append(hosts, g.collectTunnelHostRoutes(gw, config)...)
	hosts = append(hosts, g.collectUptimeKumaHostRoutes(gw, config)...)

	// forwarded_for: append 自动打标（内部PRD v1.1；v1.0.21 起回归单规则），用户无需在
	// services_biz.yaml 配置：
	//   规则 1（v1.0.19）：hostname 匹配 mesh 后缀 .{节点}.v-{env}.{mesh.domain_zone}
	//   （zone 由环境配置 mesh.yaml 提供，未配置 = 不打标）。
	// v1.2 模型下 external_backends 二级链路的上游 Host 已由自动 override_host 改写为目标侧
	// B 网关的 mesh 域名（见 buildHostRoute），B 侧命中的即 mesh 域名条目、被规则 1 覆盖；
	// v1.0.20 曾按「external_backends 服务 gateways 声明的公网 hostname 全集」打标的规则 2
	// 随 B 侧公网域名条目删除而作废移除。
	// 统一在收集完成后处理，业务路由 / 隧道路由 / uptime-kuma 路由三种来源一视同仁；
	// 未命中的 host 保持为空（gate 对未标记 host 恒为边缘清理）。
	meshZone := meshDomainZone(config)
	for i := range hosts {
		if shouldAppendForwardedFor(hosts[i].Name, g.env, meshZone) {
			hosts[i].ForwardedFor = gate.ForwardedForAppend
		}
	}

	return hosts, nil
}

// meshDomainZone 从环境配置取 mesh 域名区域（userdata/<env>/mesh.yaml 的 mesh.domain_zone）。
// 未配置 / 未填 = 空串，mesh 域名相关自动行为整体关闭。
func meshDomainZone(config *entity.Config) string {
	if config == nil || config.Mesh == nil {
		return ""
	}
	return config.Mesh.DomainZone
}

// isMeshHostname 判断 hostname 是否为 EasyTier mesh 域名（内部PRD v1.1）。
// mesh 域名格式：*.{节点}.v-{env}.{zone}（zone 见 meshDomainZone），
// 即倒数第三段为 v-{env}。env 取生成器构造时的目标环境（NewGenerator(env, ...)），
// dev/prod 各自精确匹配本环境后缀，跨环境后缀（如 dev 环境下的 *.v-prod...）不误标。
// 注意区分形态相近的外部/常规域名（均不以 .v-{env}. 段结尾，不会命中）：
//   - customer.example.cn、warden.ngc4486.com 等外部域名；
//   - hfmirror.env-dev.{zone}（倒数第三段是 env-dev 而非 v-dev，且前导字符非 .）；
//   - pay-dev.{zone}、裸 {zone}（无 v-{env} 段）。
func isMeshHostname(hostname, env, zone string) bool {
	if hostname == "" || env == "" || zone == "" {
		return false
	}
	return strings.HasSuffix(hostname, ".v-"+env+"."+zone)
}

// shouldAppendForwardedFor 判断 host 是否自动打标 forwarded_for: append（生成器内部规则，
// 非用户字段）。v1.0.21 起回归单规则（v1.0.20 的规则 2 已随 v1.2 override_host 模型移除）：
//   - 规则 1（v1.0.19）：isMeshHostname——hostname 匹配 mesh 域名后缀 .{节点}.v-{env}.{zone}。
//     external_backends 二级链路第二跳目标侧命中的 mesh 域名条目即被本规则覆盖（v1.2 模型下
//     外层代理的上游 Host 已由自动 override_host 改写为 mesh 域名，见 buildHostRoute）。
func shouldAppendForwardedFor(hostname, env, zone string) bool {
	return isMeshHostname(hostname, env, zone)
}

// collectUptimeKumaHostRoutes 为同服务器上的 uptime-kuma infra 的每个 gateway host 生成路由，
// backend 指向主容器（http://{infra.Name}:{container_port}，默认 3001），保留 Host 头
// （Kuma 依赖 Host 匹配 + UPTIME_KUMA_WS_ORIGIN_CHECK=bypass）。
func (g *Generator) collectUptimeKumaHostRoutes(gw *entity.InfraService, config *entity.Config) []gate.HostRoute {
	var hosts []gate.HostRoute
	for _, infra := range config.InfraServices {
		if infra.Type != entity.InfraServiceTypeUptimeKuma || infra.Server != gw.Server {
			continue
		}
		if infra.UptimeKumaConfig == nil {
			continue
		}
		httpPort := constants.DefaultHTTPPort
		if gw.GatewayPorts != nil {
			httpPort = gw.GatewayPorts.HTTP
		}
		for _, route := range infra.UptimeKumaConfig.Gateways {
			sslPort := 0
			if route.HTTPS && gw.GatewayPorts != nil {
				sslPort = gw.GatewayPorts.HTTPS
			}
			containerPort := route.ContainerPort
			if containerPort == 0 {
				containerPort = uptimeKumaDefaultContainerPort
			}
			hosts = append(hosts, gate.HostRoute{
				Name:                route.Hostname,
				Port:                httpPort,
				SSLPort:             sslPort,
				Backend:             []string{fmt.Sprintf("http://%s:%d", infra.Name, containerPort)},
				HealthCheck:         "/",
				HealthCheckInterval: constants.DefaultHealthInterval,
				HealthCheckTimeout:  constants.DefaultHealthTimeout,
				PreserveHostHeader:  true,
			})
		}
	}
	return hosts
}

func (g *Generator) collectTunnelHostRoutes(gw *entity.InfraService, config *entity.Config) []gate.HostRoute {
	var hosts []gate.HostRoute
	for _, infra := range config.InfraServices {
		if infra.Type != entity.InfraServiceTypeTunnelServer || infra.Server != gw.Server {
			continue
		}
		if infra.TunnelServerConfig == nil {
			continue
		}
		vhostPort := infra.TunnelServerConfig.Ports.VhostHTTPPort
		if vhostPort == 0 {
			vhostPort = defaultTunnelVhostPort
		}
		httpPort := constants.DefaultHTTPPort
		if gw.GatewayPorts != nil {
			httpPort = gw.GatewayPorts.HTTP
		}
		sslPort := 0
		if gw.GatewayPorts != nil {
			sslPort = gw.GatewayPorts.HTTPS
		}
		for _, route := range infra.TunnelServerConfig.TunnelRoutes {
			preserve := true
			disabled := false
			hosts = append(hosts, gate.HostRoute{
				Name:                route.Hostname,
				Port:                httpPort,
				SSLPort:             sslPort,
				Backend:             []string{fmt.Sprintf("http://%s:%d", infra.Name, vhostPort)},
				HealthCheck:         "/",
				HealthCheckInterval: constants.DefaultHealthInterval,
				HealthCheckTimeout:  constants.DefaultHealthTimeout,
				HealthCheckEnabled:  &disabled,
				PreserveHostHeader:  preserve,
			})
		}
	}
	return hosts
}

func (g *Generator) buildHostRoute(svc *entity.BizService, route *entity.ServiceGatewayRoute, gw *entity.InfraService, secrets map[string]string) (gate.HostRoute, error) {
	var backends []string
	if len(svc.ExternalBackends) > 0 {
		backends = svc.ExternalBackends
	} else {
		backends = []string{fmt.Sprintf("http://%s:%d", svc.Name, route.ContainerPort)}
	}

	hostname := route.Hostname
	if hostname == "" {
		hostname = svc.Name
	}

	healthPath := "/"
	if svc.Healthcheck != nil && svc.Healthcheck.Path != "" {
		healthPath = svc.Healthcheck.Path
	}

	sslPort := 0
	if route.HTTPS && gw.GatewayPorts != nil {
		sslPort = gw.GatewayPorts.HTTPS
	}

	healthInterval, healthTimeout := g.buildHealthCheckConfig(svc)

	var healthCheckEnabled *bool
	if len(svc.ExternalBackends) > 0 {
		enabled := false
		healthCheckEnabled = &enabled
	}

	httpPort := constants.DefaultHTTPPort
	if gw.GatewayPorts != nil {
		httpPort = gw.GatewayPorts.HTTP
	}

	var wafMode string
	var wafRequestBodyAccess *bool
	if route.WAF != nil {
		wafMode = route.WAF.Mode
		wafRequestBodyAccess = route.WAF.RequestBodyAccess
	}

	// v1.2（v1.0.21）：external_backends 服务未显式配置 override_host 时，自动将上游请求的
	// Host 改写为 external_backends URL 的 host（目标侧 B 网关的 mesh 域名），B 按 mesh 域名
	// 条目路由（规则 1 已覆盖其 forwarded_for append 打标）。显式配置值优先、不覆盖（prod
	// 存量 token-plan 伪装第三方案已显式配置 override_host，不受影响；未显式配置的伪装案
	// 自动 override 为第三方 host，与「作为第三方 host 访问」的伪装语义一致）；URL host 不
	// 统一或解析失败时保持现状不 override，仅输出告警日志（不阻断生成）。
	overrideHost := route.OverrideHost
	if len(svc.ExternalBackends) > 0 && overrideHost == "" {
		if backendHost, ok := externalBackendsOverrideHost(svc.ExternalBackends); ok {
			overrideHost = backendHost
		} else {
			logger.Warn("external_backends 的 URL host 不统一或 URL 非法，跳过自动 override_host（该 host 保持原 Host 转发）",
				"env", g.env, "service", svc.Name, "hostname", hostname,
				"external_backends", svc.ExternalBackends)
		}
	}

	// 密码门（内部PRD）：auth.password 的 secret 引用在生成阶段解析，
	// Resolve 失败 → 生成报错，不产出半成品配置（避免"以为上锁了实际没锁"的
	// 半成品 gate.yaml 下发）。未配置 auth 的 host 三个字段保持零值，不生成 auth 段。
	var authPassword, authTTL string
	var authPublicPaths []string
	var authMaxFailures int
	var authLockDuration string
	if route.Auth != nil {
		password, err := route.Auth.Password.Resolve(secrets)
		if err != nil {
			return gate.HostRoute{}, fmt.Errorf("resolve auth.password: %w", err)
		}
		authPassword = password
		authTTL = route.Auth.TTL
		authPublicPaths = route.Auth.PublicPaths
		authMaxFailures = route.Auth.MaxFailures   // 0 = 未配置，产物不生成该键（gate 缺省 5）
		authLockDuration = route.Auth.LockDuration // 空 = 未配置，产物不生成该键（gate 缺省 15m）
	}

	return gate.HostRoute{
		Name:                 hostname,
		Port:                 httpPort,
		SSLPort:              sslPort,
		Backend:              backends,
		HealthCheck:          healthPath,
		HealthCheckInterval:  healthInterval,
		HealthCheckTimeout:   healthTimeout,
		HealthCheckEnabled:   healthCheckEnabled,
		PreserveHostHeader:   true,
		GZipEnabled:          route.GzipEnabled,
		OverrideHost:         overrideHost,
		StripProxyHeaders:    route.StripProxyHeaders,
		RobotsDenyAll:        route.RobotsDenyAll,
		AllowedIPs:           route.AllowedIPs,
		WAFMode:              wafMode,
		WAFRequestBodyAccess: wafRequestBodyAccess,
		BlacklistMode:        route.BlacklistMode,
		AuthPassword:         authPassword,
		AuthTTL:              authTTL,
		AuthPublicPaths:      authPublicPaths,
		// M3 防爆破参数（内部PRD）：0/空 = 未配置，产物不生成对应键（gate 缺省 5 次/15m）
		AuthMaxFailures:  authMaxFailures,
		AuthLockDuration: authLockDuration,
		// L3 per-host WSS 上游证书校验跳过（内部PRD）：nil = 未配置不生成该键（gate 继承全局）
		WebSocketInsecureSkip: route.WebSocketInsecureSkip,
		// 逐条目 http→https 301 + 规范域收敛 + HSTS per-host（内部总体方案 W3-2 / D8），
		// 三字段原样透传，未配置保持零值（产物不生成对应键，存量输出逐字节不变）
		RedirectHTTPS: route.RedirectHTTPS,
		RedirectHost:  route.RedirectHost,
		HSTS:          route.HSTS,
	}, nil
}

// externalBackendsOverrideHost 解析 external_backends URL 集合，返回可作为自动 override_host
// 的统一上游 host（内部PRD v1.2）。取 net/url 解析后的 URL host（含显式端口，与
// 「直接以该 URL 访问」时的 Host 头语义一致）：
//   - 全部 URL 解析成功且 host 相同（正常情况仅 1 个 URL）→ (host, true)；
//   - 多个 URL 且 host 不同，或任一 URL 解析失败 / host 为空 → ("", false)，
//     调用方保持现状不 override（与 v1.0.20 及之前的行为一致）并告警。
func externalBackendsOverrideHost(backends []string) (string, bool) {
	host := ""
	for _, raw := range backends {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return "", false
		}
		if host == "" {
			host = u.Host
			continue
		}
		if host != u.Host {
			return "", false
		}
	}
	if host == "" {
		return "", false
	}
	return host, true
}

func (g *Generator) buildHealthCheckConfig(svc *entity.BizService) (string, string) {
	healthInterval := constants.DefaultHealthInterval
	healthTimeout := constants.DefaultHealthTimeout
	if svc.Healthcheck != nil {
		if svc.Healthcheck.Interval != "" {
			healthInterval = svc.Healthcheck.Interval
		}
		if svc.Healthcheck.Timeout != "" {
			healthTimeout = svc.Healthcheck.Timeout
		}
	}
	return healthInterval, healthTimeout
}

func (g *Generator) buildGatewayConfig(gw *entity.InfraService) *gate.GatewayConfig {
	wafEnabled := false
	wafDefaultMode := ""
	var whitelist []string
	var crsProxyURL string
	var crsMirrorURL string
	var crsFailurePolicy string
	var crsPinnedSHA256 string
	var crsMirrorAllowInsecureHTTP *bool
	sslMode := ""
	sslEndpoint := ""
	sslAPIKey := ""

	if gw.GatewayWAF != nil {
		wafEnabled = gw.GatewayWAF.Enabled
		wafDefaultMode = gw.GatewayWAF.DefaultMode
		whitelist = gw.GatewayWAF.Whitelist
		crsProxyURL = gw.GatewayWAF.CRSProxyURL
		crsMirrorURL = gw.GatewayWAF.CRSMirrorURL
		crsFailurePolicy = gw.GatewayWAF.CRSFailurePolicy
		crsPinnedSHA256 = gw.GatewayWAF.CRSPinnedSHA256                       // 空 = 未配置，产物不生成该键
		crsMirrorAllowInsecureHTTP = gw.GatewayWAF.CRSMirrorAllowInsecureHTTP // nil = 未配置，产物不生成该键
	}
	if gw.GatewaySSL != nil {
		sslMode = gw.GatewaySSL.Mode
		sslEndpoint = gw.GatewaySSL.Endpoint
		sslAPIKey = gw.GatewaySSL.APIKey
	}

	httpPort := constants.DefaultHTTPPort
	if gw.GatewayPorts != nil {
		httpPort = gw.GatewayPorts.HTTP
	}

	config := &gate.GatewayConfig{
		Port:                       httpPort,
		LogLevel:                   gw.GatewayLogLevel,
		WAFEnabled:                 wafEnabled,
		WAFDefaultMode:             wafDefaultMode,
		Whitelist:                  whitelist,
		CRSProxyURL:                crsProxyURL,
		CRSMirrorURL:               crsMirrorURL,
		CRSFailurePolicy:           crsFailurePolicy, // 空 = 未配置，产物不生成 failure_policy 键
		CRSPinnedSHA256:            crsPinnedSHA256,  // 空 = 未配置，产物不生成 pinned_sha256 键
		CRSMirrorAllowInsecureHTTP: crsMirrorAllowInsecureHTTP,
		TrustedProxies:             gw.GatewayTrustedProxies,
		// H1 路径规范化强制逃生口 + L3 全局 WSS 跳过校验（内部PRD）：
		// nil = 未配置，产物不生成对应键（gate 侧取安全缺省）
		StrictPathCheck:       gw.GatewayStrictPathCheck,
		WebSocketInsecureSkip: gw.GatewayWebSocketInsecureSkip,
		SSLMode:               sslMode,
		SSLEndpoint:           sslEndpoint,
		SSLAPIKey:             sslAPIKey,
	}

	// Map blacklist config if provided
	if gw.GatewayBlacklist != nil {
		config.Blacklist = &gate.BlacklistConfig{
			Enabled:         gw.GatewayBlacklist.Enabled,
			DefaultMode:     gw.GatewayBlacklist.DefaultMode,
			Whitelist:       gw.GatewayBlacklist.Whitelist,
			SourceURL:       gw.GatewayBlacklist.SourceURL,
			AllowHTTPSource: gw.GatewayBlacklist.AllowHTTPSource, // nil = 未配置，产物不生成该键
			MaxEntries:      gw.GatewayBlacklist.MaxEntries,      // 0 = 未配置，产物不生成该键
		}
	}

	// Map notification config if provided
	if gw.GatewayNotification != nil {
		config.Notification = &gate.NotificationConfig{
			Enabled: gw.GatewayNotification.Enabled,
			URL:     gw.GatewayNotification.URL,
			Timeout: gw.GatewayNotification.Timeout,
		}
	}

	return config
}

// buildTrackerShipperConfig 映射 gateway 实体的 tracker_shipper 到 gate 顶层段
// （内部PRD §7.1）。复用 auth.password 的 secret 解析先例：{secret: name} 引用在
// 生成阶段解析为明文，找不到 secret 直接报错，不产出半成品 gate.yaml。
// node_name 显式配置值优先；未配置时按服务命名规范自动注入容器名 yo-{env}-{服务名}
// （服务名取 gateway infra service 的 name，如 gateway-node1 → yo-dev-gateway-node1）。
// buffer_size/batch_size/flush_interval/failure_sample_log_ratio 未配置时填 gate 侧缺省值，
// 产物完整自描述。nil = 未配置，产物不生成 tracker_shipper 键（存量输出逐字节不变）。
func (g *Generator) buildTrackerShipperConfig(gw *entity.InfraService, secrets map[string]string) (*gate.TrackerShipperConfig, error) {
	src := gw.GatewayTrackerShipper
	if src == nil {
		return nil, nil
	}

	secret, err := src.SharedSecret.Resolve(secrets)
	if err != nil {
		return nil, fmt.Errorf("resolve tracker_shipper.shared_secret: %w", err)
	}

	nodeName := src.NodeName
	if nodeName == "" {
		nodeName = fmt.Sprintf("yo-%s-%s", g.env, gw.Name)
	}

	bufferSize := src.BufferSize
	if bufferSize == 0 {
		bufferSize = gate.DefaultTrackerShipperBufferSize
	}
	batchSize := src.BatchSize
	if batchSize == 0 {
		batchSize = gate.DefaultTrackerShipperBatchSize
	}
	flushInterval := src.FlushInterval
	if flushInterval == "" {
		flushInterval = gate.DefaultTrackerShipperFlushInterval
	}
	failureSampleLogRatio := src.FailureSampleLogRatio
	if failureSampleLogRatio == 0 {
		failureSampleLogRatio = gate.DefaultTrackerShipperFailureSampleLogRatio
	}

	return &gate.TrackerShipperConfig{
		Enabled:               src.Enabled,
		Endpoint:              src.Endpoint,
		SharedSecret:          secret,
		NodeName:              nodeName,
		BufferSize:            bufferSize,
		BatchSize:             batchSize,
		FlushInterval:         flushInterval,
		FailureSampleLogRatio: failureSampleLogRatio,
	}, nil
}

func (g *Generator) generateGatewayConfig(serverDir string, gw *entity.InfraService, config *entity.Config) error {
	result, err := g.buildGatewayRoutes(gw, config)
	if err != nil {
		return fmt.Errorf("failed to build gateway routes for %s: %w", gw.Name, err)
	}

	content, err := g.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		return fmt.Errorf("failed to generate gateway config for %s: %w", gw.Name, err)
	}

	configFile := filepath.Join(serverDir, fmt.Sprintf("%s.gate.yaml", gw.Name))
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write gateway config file %s: %w", configFile, err)
	}

	composeContent, err := g.generateGatewayCompose(serverDir, gw, result.httpPort, result.httpsPort, config)
	if err != nil {
		return fmt.Errorf("failed to generate gateway compose for %s: %w", gw.Name, err)
	}

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", gw.Name))
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("failed to write gateway compose file %s: %w", composeFile, err)
	}

	return nil
}

func (g *Generator) generateGatewayCompose(serverDir string, gw *entity.InfraService, httpPort, httpsPort int, config *entity.Config) (string, error) {
	networkName := "yamlops-" + g.env

	ports := []string{
		fmt.Sprintf("%d:%d", httpPort, httpPort),
		fmt.Sprintf("%d:%d", httpsPort, httpsPort),
	}
	for _, p := range gw.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d", p.Host, p.Container))
	}

	volumes := []string{
		constants.GatewayConfigPath,
		constants.GatewayCachePath,
	}

	envMap := make(map[string]string)
	secrets := config.GetSecretsMap()
	for k, ref := range gw.Env {
		val, err := ref.Resolve(secrets)
		if err != nil {
			return "", fmt.Errorf("failed to resolve env %s for gateway %s: %w", k, gw.Name, err)
		}
		envMap[k] = val
	}
	for _, secretName := range gw.Secrets {
		if val, ok := secrets[secretName]; ok {
			envMap[strings.ToUpper(secretName)] = val
		}
	}

	serverMap := config.GetServerMap()
	if server, ok := serverMap[gw.Server]; ok {
		envMap["DEPLOY_ZONE_NAME"] = server.Zone
	}
	envMap["DEPLOY_ENV_NAME"] = g.env
	envMap["DEPLOY_SERVER_NAME"] = gw.Server
	envMap["DEPLOY_SERVICE_NAME"] = gw.Name

	envFileName := fmt.Sprintf("%s.env", gw.Name)
	envFile := filepath.Join(serverDir, envFileName)

	envLines := []string{}
	for k, v := range envMap {
		envLines = append(envLines, fmt.Sprintf("%s=%s", k, v))
	}
	envContent := strings.Join(envLines, "\n") + "\n"

	if err := os.WriteFile(envFile, []byte(envContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write env file %s: %w", envFile, err)
	}

	composeSvc := &compose.ComposeService{
		Name:       gw.Name,
		Image:      gw.Image,
		Ports:      ports,
		EnvFiles:   []string{envFileName},
		Volumes:    volumes,
		Networks:   []string{networkName},
		ExtraHosts: []string{constants.HostDockerGateway},
	}

	return g.composeGen.Generate(composeSvc, g.env)
}

func (g *Generator) generateInfraGatewayConfig(infra *entity.InfraService, config *entity.Config) (string, error) {
	result, err := g.buildGatewayRoutes(infra, config)
	if err != nil {
		return "", err
	}
	return g.gateGen.Generate(result.gatewayConfig, result.hosts)
}
