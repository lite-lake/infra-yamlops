package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/application/generator/compose"
	"github.com/lite-lake/infra-yamlops/internal/application/generator/gate"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
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

func (g *Generator) buildGatewayRoutes(gw *entity.InfraService, config *entity.Config) *gatewayRouteResult {
	hosts := g.collectHostRoutes(gw, config)
	gatewayConfig := g.buildGatewayConfig(gw)

	return &gatewayRouteResult{
		hosts:         hosts,
		gatewayConfig: gatewayConfig,
		httpPort:      gatewayConfig.Port,
		httpsPort:     constants.DefaultHTTPSPort,
	}
}

func (g *Generator) collectHostRoutes(gw *entity.InfraService, config *entity.Config) []gate.HostRoute {
	var hosts []gate.HostRoute
	for _, svc := range config.Services {
		if svc.Server != gw.Server {
			continue
		}
		for _, route := range svc.Gateways {
			if !route.HasGateway() {
				continue
			}
			hosts = append(hosts, g.buildHostRoute(&svc, &route, gw))
		}
	}
	return hosts
}

func (g *Generator) buildHostRoute(svc *entity.BizService, route *entity.ServiceGatewayRoute, gw *entity.InfraService) gate.HostRoute {
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

	var wafDisabled *bool
	if route.WAF != nil && route.WAF.Enabled != nil && !*route.WAF.Enabled {
		t := true
		wafDisabled = &t
	}

	return gate.HostRoute{
		Name:                hostname,
		Port:                httpPort,
		SSLPort:             sslPort,
		Backend:             backends,
		HealthCheck:         healthPath,
		HealthCheckInterval: healthInterval,
		HealthCheckTimeout:  healthTimeout,
		HealthCheckEnabled:  healthCheckEnabled,
		PreserveHostHeader:  true,
		GZipEnabled:         route.GzipEnabled,
		OverrideHost:        route.OverrideHost,
		StripProxyHeaders:   route.StripProxyHeaders,
		WAFDisabled:         wafDisabled,
	}
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
	var whitelist []string
	sslMode := ""
	sslEndpoint := ""
	sslAPIKey := ""

	if gw.GatewayWAF != nil {
		wafEnabled = gw.GatewayWAF.Enabled
		whitelist = gw.GatewayWAF.Whitelist
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
		Port:        httpPort,
		LogLevel:    gw.GatewayLogLevel,
		WAFEnabled:  wafEnabled,
		Whitelist:   whitelist,
		SSLMode:     sslMode,
		SSLEndpoint: sslEndpoint,
		SSLAPIKey:   sslAPIKey,
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

func (g *Generator) generateGatewayConfig(serverDir string, gw *entity.InfraService, config *entity.Config) error {
	result := g.buildGatewayRoutes(gw, config)

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
	result := g.buildGatewayRoutes(infra, config)
	return g.gateGen.Generate(result.gatewayConfig, result.hosts)
}
