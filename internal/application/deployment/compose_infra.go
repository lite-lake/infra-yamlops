package deployment

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
)

func (g *Generator) generateInfraServiceComposes(config *entity.Config) error {
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

func (g *Generator) generateInfraServiceCompose(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	switch infra.Type {
	case entity.InfraServiceTypeGateway:
		return g.generateInfraServiceGateway(serverDir, infra, config)
	case entity.InfraServiceTypeSSL:
		return g.generateInfraServiceSSL(serverDir, infra, config)
	}
	return nil
}

func (g *Generator) generateInfraServiceGateway(serverDir string, infra *entity.InfraService, config *entity.Config) error {
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

func (g *Generator) generateInfraGatewayCompose(infra *entity.InfraService) string {
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

func (g *Generator) generateInfraServiceSSL(serverDir string, infra *entity.InfraService, config *entity.Config) error {
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
