package plan

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/gate"
)

func (g *deploymentGenerator) generateGatewayConfigs(config *entity.Config) error {
	gatewayServers := make(map[string][]*entity.Gateway)
	for i := range config.Gateways {
		gw := &config.Gateways[i]
		gatewayServers[gw.Server] = append(gatewayServers[gw.Server], gw)
	}

	for serverName, gateways := range gatewayServers {
		serverDir := filepath.Join(g.outputDir, serverName)
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

func (g *deploymentGenerator) generateGatewayConfig(serverDir string, gw *entity.Gateway, config *entity.Config) error {
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
				backendIP = "127.0.0.1"
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

			healthPath := "/health"
			if svc.Healthcheck != nil {
				healthPath = svc.Healthcheck.Path
			}

			sslPort := 0
			if route.HTTPS {
				sslPort = gw.Ports.HTTPS
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
				Port:                gw.Ports.HTTP,
				SSLPort:             sslPort,
				Backend:             []string{backend},
				HealthCheck:         healthPath,
				HealthCheckInterval: healthInterval,
				HealthCheckTimeout:  healthTimeout,
			})
		}
	}

	gatewayConfig := &gate.GatewayConfig{
		Port:               gw.Ports.HTTP,
		LogLevel:           gw.LogLevel,
		WAFEnabled:         gw.WAF.Enabled,
		Whitelist:          gw.WAF.Whitelist,
		SSLMode:            gw.SSL.Mode,
		SSLEndpoint:        gw.SSL.Endpoint,
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

	composeContent, err := g.generateGatewayCompose(gw)
	if err != nil {
		return fmt.Errorf("failed to generate gateway compose for %s: %w", gw.Name, err)
	}

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", gw.Name))
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("failed to write gateway compose file %s: %w", composeFile, err)
	}

	return nil
}

func (g *deploymentGenerator) generateGatewayCompose(gw *entity.Gateway) (string, error) {
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
    networks:
      - %s

networks:
  %s:
    external: true
`, serviceName, gw.Image, serviceName, gw.Ports.HTTP, gw.Ports.HTTP, gw.Ports.HTTPS, gw.Ports.HTTPS, networkName, networkName)

	return compose, nil
}
