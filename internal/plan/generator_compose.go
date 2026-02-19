package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/compose"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/gate"
)

type deploymentGenerator struct {
	composeGen *compose.Generator
	gateGen    *gate.Generator
	outputDir  string
	env        string
}

func newDeploymentGenerator(env, outputDir string) *deploymentGenerator {
	return &deploymentGenerator{
		composeGen: compose.NewGenerator(),
		gateGen:    gate.NewGenerator(),
		outputDir:  outputDir,
		env:        env,
	}
}

func (g *deploymentGenerator) generate(config *entity.Config) error {
	if _, err := os.Stat(g.outputDir); err == nil {
		if err := os.RemoveAll(g.outputDir); err != nil {
			return fmt.Errorf("failed to clean output directory: %w", err)
		}
	}

	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := g.generateServiceComposes(config); err != nil {
		return err
	}

	if err := g.generateGatewayConfigs(config); err != nil {
		return err
	}

	if err := g.generateInfraServiceComposes(config); err != nil {
		return err
	}

	return nil
}

func (g *deploymentGenerator) generateInfraServiceComposes(config *entity.Config) error {
	serverInfraServices := make(map[string][]*entity.InfraService)
	for i := range config.InfraServices {
		infra := &config.InfraServices[i]
		serverInfraServices[infra.Server] = append(serverInfraServices[infra.Server], infra)
	}

	for serverName, infraServices := range serverInfraServices {
		serverDir := filepath.Join(g.outputDir, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverName, err)
		}

		for _, infra := range infraServices {
			if err := g.generateInfraServiceCompose(serverDir, infra, config); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *deploymentGenerator) generateInfraServiceCompose(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	switch infra.Type {
	case entity.InfraServiceTypeGateway:
		return g.generateInfraServiceGateway(serverDir, infra, config)
	case entity.InfraServiceTypeSSL:
		return g.generateInfraServiceSSL(serverDir, infra, config)
	}
	return nil
}

func (g *deploymentGenerator) generateInfraServiceGateway(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	if infra.GatewayPorts != nil {
		composeContent := g.generateInfraGatewayCompose(infra)
		composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", infra.Name))
		if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
			return fmt.Errorf("failed to write infra compose file %s: %w", composeFile, err)
		}

		gatewayContent, err := g.generateInfraGatewayConfig(infra, config)
		if err != nil {
			return fmt.Errorf("failed to generate gateway config for %s: %w", infra.Name, err)
		}
		gatewayFile := filepath.Join(serverDir, fmt.Sprintf("%s.gate.yaml", infra.Name))
		if err := os.WriteFile(gatewayFile, []byte(gatewayContent), 0644); err != nil {
			return fmt.Errorf("failed to write gateway config file %s: %w", gatewayFile, err)
		}
	}
	return nil
}

func (g *deploymentGenerator) generateInfraGatewayCompose(infra *entity.InfraService) string {
	serviceName := "yo-" + g.env + "-" + infra.Name
	networkName := "yamlops-" + g.env

	return fmt.Sprintf(`services:
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
`, serviceName, infra.Image, serviceName, infra.GatewayPorts.HTTP, infra.GatewayPorts.HTTP, infra.GatewayPorts.HTTPS, infra.GatewayPorts.HTTPS, networkName, networkName)
}

func (g *deploymentGenerator) generateInfraGatewayConfig(infra *entity.InfraService, config *entity.Config) (string, error) {
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

func (g *deploymentGenerator) generateInfraServiceSSL(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	if infra.SSLConfig == nil {
		return nil
	}

	serviceName := "yo-" + g.env + "-" + infra.Name
	networkName := "yamlops-" + g.env

	volumes := []string{
		"./ssl-data:/app/data",
		"./ssl-config:/app/configs",
	}
	namedVolumes := []string{}
	for _, v := range infra.SSLConfig.Volumes {
		converted := convertVolumeProtocol(v)
		volumes = append(volumes, converted)
		if volName := extractNamedVolume(converted); volName != "" {
			namedVolumes = append(namedVolumes, volName)
		}
	}

	ports := []string{}
	if infra.SSLConfig.Ports.API > 0 {
		ports = append(ports, fmt.Sprintf("%d:%d", infra.SSLConfig.Ports.API, infra.SSLConfig.Ports.API))
	}

	volumesSection := ""
	if len(namedVolumes) > 0 {
		volumesSection = "\nvolumes:\n"
		for _, vn := range namedVolumes {
			volumesSection += fmt.Sprintf("  %s:\n    external: true\n", vn)
		}
	}

	composeContent := fmt.Sprintf(`services:
  %s:
    image: %s
    container_name: %s
    restart: unless-stopped
    ports:
      - "%s"
    volumes:
      - %s
    networks:
      - %s

networks:
  %s:
    external: true
%s`, serviceName, infra.Image, serviceName, strings.Join(ports, "\n      - "), strings.Join(volumes, "\n      - "), networkName, networkName, volumesSection)

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", infra.Name))
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("failed to write ssl compose file %s: %w", composeFile, err)
	}

	sslConfigContent, err := g.generateSSLConfig(infra, config)
	if err != nil {
		return fmt.Errorf("failed to generate ssl config for %s: %w", infra.Name, err)
	}
	sslConfigDir := filepath.Join(serverDir, "ssl-config")
	if err := os.MkdirAll(sslConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create ssl-config directory: %w", err)
	}
	sslConfigFile := filepath.Join(sslConfigDir, "config.yml")
	if err := os.WriteFile(sslConfigFile, []byte(sslConfigContent), 0644); err != nil {
		return fmt.Errorf("failed to write ssl config file %s: %w", sslConfigFile, err)
	}

	return nil
}

func (g *deploymentGenerator) generateSSLConfig(infra *entity.InfraService, config *entity.Config) (string, error) {
	ssl := infra.SSLConfig
	secrets := config.GetSecretsMap()

	apiKey, err := ssl.Auth.APIKey.Resolve(secrets)
	if err != nil {
		return "", fmt.Errorf("failed to resolve apikey: %w", err)
	}

	return fmt.Sprintf(`auth:
  enabled: %t
  apikey: %s
storage:
  type: %s
  path: %s
defaults:
  issue_provider: %s
  storage_provider: %s
`, ssl.Auth.Enabled, apiKey, ssl.Storage.Type, ssl.Storage.Path, ssl.Defaults.IssueProvider, ssl.Defaults.StorageProvider), nil
}

func (g *deploymentGenerator) generateServiceComposes(config *entity.Config) error {
	serverServices := make(map[string][]*entity.BizService)
	for i := range config.Services {
		svc := &config.Services[i]
		serverServices[svc.Server] = append(serverServices[svc.Server], svc)
	}

	for serverName, services := range serverServices {
		serverDir := filepath.Join(g.outputDir, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverDir, err)
		}

		for _, svc := range services {
			if err := g.generateServiceCompose(serverDir, svc, config.GetSecretsMap()); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *deploymentGenerator) generateServiceCompose(serverDir string, svc *entity.BizService, secrets map[string]string) error {
	ports := []string{}
	for _, port := range svc.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d", port.Host, port.Container))
	}

	volumes := []string{}
	for _, v := range svc.Volumes {
		volumes = append(volumes, fmt.Sprintf("%s:%s", v.Source, v.Target))
	}

	var healthCheck *compose.HealthCheck
	if svc.Healthcheck != nil {
		healthCheck = &compose.HealthCheck{
			Test:     []string{"CMD", "curl", "-f", svc.Healthcheck.Path},
			Interval: svc.Healthcheck.Interval,
			Timeout:  svc.Healthcheck.Timeout,
			Retries:  3,
		}
	}

	var resources *compose.Resources
	if svc.Resources.CPU != "" || svc.Resources.Memory != "" {
		resources = &compose.Resources{
			Limits: &compose.ResourceLimits{
				Cpus:   svc.Resources.CPU,
				Memory: svc.Resources.Memory,
			},
		}
	}

	envMap := make(map[string]string)
	for k, ref := range svc.Env {
		val, err := ref.Resolve(secrets)
		if err != nil {
			return fmt.Errorf("failed to resolve env %s for service %s: %w", k, svc.Name, err)
		}
		envMap[k] = val
	}
	for _, secretName := range svc.Secrets {
		if val, ok := secrets[secretName]; ok {
			envKey := strings.ToUpper(secretName)
			envMap[envKey] = val
		}
	}

	composeSvc := &compose.ComposeService{
		Name:        svc.Name,
		Image:       svc.Image,
		Ports:       ports,
		Environment: envMap,
		Volumes:     volumes,
		HealthCheck: healthCheck,
		Resources:   resources,
		Internal:    svc.Internal,
	}

	content, err := g.composeGen.Generate(composeSvc, g.env)
	if err != nil {
		return fmt.Errorf("failed to generate compose for service %s: %w", svc.Name, err)
	}

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", svc.Name))
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write compose file %s: %w", composeFile, err)
	}

	return nil
}

func convertVolumeProtocol(v string) string {
	return strings.Replace(v, "volumes://", "./", 1)
}

func extractNamedVolume(v string) string {
	if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "/") {
		return ""
	}
	parts := strings.SplitN(v, ":", 2)
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
