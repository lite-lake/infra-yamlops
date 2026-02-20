package deployment

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/gate"
)

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

func (g *Generator) generateGatewayConfig(serverDir string, gw *entity.InfraService, config *entity.Config) error {
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
	logLevel := 0

	if gw.GatewayWAF != nil {
		wafEnabled = gw.GatewayWAF.Enabled
		whitelist = gw.GatewayWAF.Whitelist
	}
	if gw.GatewaySSL != nil {
		sslMode = gw.GatewaySSL.Mode
		sslEndpoint = gw.GatewaySSL.Endpoint
	}
	logLevel = gw.GatewayLogLevel

	httpPort := 80
	httpsPort := 443
	if gw.GatewayPorts != nil {
		httpPort = gw.GatewayPorts.HTTP
		httpsPort = gw.GatewayPorts.HTTPS
	}

	gatewayConfig := &gate.GatewayConfig{
		Port:               httpPort,
		LogLevel:           logLevel,
		WAFEnabled:         wafEnabled,
		Whitelist:          whitelist,
		SSLMode:            sslMode,
		SSLEndpoint:        sslEndpoint,
		SSLAutoUpdate:      true,
		SSLUpdateCheckTime: "00:00-00:59",
	}

	content, err := g.gateGen.Generate(gatewayConfig, hosts)
	if err != nil {
		return fmt.Errorf("failed to generate gateway config for %s: %w", gw.Name, err)
	}

	configFile := filepath.Join(serverDir, fmt.Sprintf("%s.gate.yaml", gw.Name))
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write gateway config file %s: %w", configFile, err)
	}

	composeContent, err := g.generateGatewayCompose(gw, httpPort, httpsPort)
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
	serviceName := "yo-" + g.env + "-" + gw.Name
	networkName := "yamlops-" + g.env

	compose := fmt.Sprintf(`services:
  %s:
    image: %s
    container_name: %s
    restart: unless-stopped
    ports:
      - "%d:%d"
      - "%d:%d"
    volumes:
      - ./gateway.yml:/app/configs/server.yml:ro
      - ./cache:/app/cache
    extra_hosts:
      - "host.docker.internal:host-gateway"
    networks:
      - %s

networks:
  %s:
    external: true
`, serviceName, gw.Image, serviceName, httpPort, httpPort, httpsPort, httpsPort, networkName, networkName)

	return compose, nil
}

func (g *Generator) generateInfraGatewayConfig(infra *entity.InfraService, config *entity.Config) (string, error) {
	var hosts []gate.HostRoute

	serverMap := config.GetServerMap()
	containerPortToHostPort := make(map[string]int)
	for _, svc := range config.Services {
		if svc.Server == infra.Server {
			for _, port := range svc.Ports {
				key := fmt.Sprintf("%s:%d", svc.Name, port.Container)
				containerPortToHostPort[key] = port.Host
			}
		}
	}

	for _, svc := range config.Services {
		if svc.Server != infra.Server {
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
			if route.HTTPS && infra.GatewayPorts != nil {
				sslPort = infra.GatewayPorts.HTTPS
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

			hosts = append(hosts, gate.HostRoute{
				Name:                hostname,
				Port:                infra.GatewayPorts.HTTP,
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
	sslMode := "local"
	var sslEndpoint string
	if infra.GatewayWAF != nil {
		wafEnabled = infra.GatewayWAF.Enabled
		whitelist = infra.GatewayWAF.Whitelist
	}
	if infra.GatewaySSL != nil {
		sslMode = infra.GatewaySSL.Mode
		sslEndpoint = infra.GatewaySSL.Endpoint
	}

	gatewayConfig := &gate.GatewayConfig{
		Port:               infra.GatewayPorts.HTTP,
		LogLevel:           infra.GatewayLogLevel,
		WAFEnabled:         wafEnabled,
		Whitelist:          whitelist,
		SSLMode:            sslMode,
		SSLEndpoint:        sslEndpoint,
		SSLAutoUpdate:      true,
		SSLUpdateCheckTime: "00:00-00:59",
	}

	return g.gateGen.Generate(gatewayConfig, hosts)
}
