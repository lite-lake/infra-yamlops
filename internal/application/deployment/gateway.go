package deployment

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/infrastructure/generator/compose"
	"github.com/litelake/yamlops/internal/infrastructure/generator/gate"
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
		serverDir := filepath.Join(g.outputDir, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverName, err)
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
	serverMap := config.GetServerMap()
	var hosts []gate.HostRoute

	containerPortToHostPort := make(map[string]int)
	for _, svc := range config.Services {
		if svc.Server == gw.Server {
			for _, port := range svc.Ports {
				key := fmt.Sprintf("%s:%d", svc.Name, port.Container)
				containerPortToHostPort[key] = port.Host
			}
		}
	}

	for _, svc := range config.Services {
		if svc.Server != gw.Server {
			continue
		}
		for _, route := range svc.Gateways {
			if !route.HasGateway() {
				continue
			}

			var backendIP string
			if server, ok := serverMap[svc.Server]; ok && server.IP.Private != "" {
				backendIP = server.IP.Private
			} else {
				backendIP = "host.docker.internal"
			}

			hostPort := route.ContainerPort
			if key := fmt.Sprintf("%s:%d", svc.Name, route.ContainerPort); containerPortToHostPort[key] > 0 {
				hostPort = containerPortToHostPort[key]
			}
			backend := fmt.Sprintf("http://%s:%d", backendIP, hostPort)

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

			healthInterval := "30s"
			healthTimeout := "10s"
			if svc.Healthcheck != nil {
				if svc.Healthcheck.Interval != "" {
					healthInterval = svc.Healthcheck.Interval
				}
				if svc.Healthcheck.Timeout != "" {
					healthTimeout = svc.Healthcheck.Timeout
				}
			}

			httpPort := 80
			if gw.GatewayPorts != nil {
				httpPort = gw.GatewayPorts.HTTP
			}

			hosts = append(hosts, gate.HostRoute{
				Name:                hostname,
				Port:                httpPort,
				SSLPort:             sslPort,
				Backend:             []string{backend},
				HealthCheck:         healthPath,
				HealthCheckInterval: healthInterval,
				HealthCheckTimeout:  healthTimeout,
			})
		}
	}

	wafEnabled := false
	var whitelist []string
	sslMode := ""
	sslEndpoint := ""

	if gw.GatewayWAF != nil {
		wafEnabled = gw.GatewayWAF.Enabled
		whitelist = gw.GatewayWAF.Whitelist
	}
	if gw.GatewaySSL != nil {
		sslMode = gw.GatewaySSL.Mode
		sslEndpoint = gw.GatewaySSL.Endpoint
	}

	httpPort := 80
	httpsPort := 443
	if gw.GatewayPorts != nil {
		httpPort = gw.GatewayPorts.HTTP
		httpsPort = gw.GatewayPorts.HTTPS
	}

	gatewayConfig := &gate.GatewayConfig{
		Port:               httpPort,
		LogLevel:           gw.GatewayLogLevel,
		WAFEnabled:         wafEnabled,
		Whitelist:          whitelist,
		SSLMode:            sslMode,
		SSLEndpoint:        sslEndpoint,
		SSLAutoUpdate:      true,
		SSLUpdateCheckTime: "00:00-00:59",
	}

	return &gatewayRouteResult{
		hosts:         hosts,
		gatewayConfig: gatewayConfig,
		httpPort:      httpPort,
		httpsPort:     httpsPort,
	}
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

	composeContent, err := g.generateGatewayCompose(gw, result.httpPort, result.httpsPort)
	if err != nil {
		return fmt.Errorf("failed to generate gateway compose for %s: %w", gw.Name, err)
	}

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", gw.Name))
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("failed to write gateway compose file %s: %w", composeFile, err)
	}

	return nil
}

func (g *Generator) generateGatewayCompose(gw *entity.InfraService, httpPort, httpsPort int) (string, error) {
	networkName := "yamlops-" + g.env

	ports := []string{
		fmt.Sprintf("%d:%d", httpPort, httpPort),
		fmt.Sprintf("%d:%d", httpsPort, httpsPort),
	}

	volumes := []string{
		"./gateway.yml:/app/configs/server.yml:ro",
		"./cache:/app/cache",
	}

	composeSvc := &compose.ComposeService{
		Name:       gw.Name,
		Image:      gw.Image,
		Ports:      ports,
		Volumes:    volumes,
		Networks:   []string{networkName},
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	return g.composeGen.Generate(composeSvc, g.env)
}

func (g *Generator) generateInfraGatewayConfig(infra *entity.InfraService, config *entity.Config) (string, error) {
	result := g.buildGatewayRoutes(infra, config)
	return g.gateGen.Generate(result.gatewayConfig, result.hosts)
}
