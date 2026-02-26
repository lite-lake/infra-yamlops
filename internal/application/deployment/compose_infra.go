package deployment

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

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
		composeContent, err := g.generateInfraGatewayCompose(infra)
		if err != nil {
			return fmt.Errorf("failed to generate gateway compose for %s: %w", infra.Name, err)
		}
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

func (g *Generator) generateInfraGatewayCompose(infra *entity.InfraService) (string, error) {
	serviceName := "yo-" + g.env + "-" + infra.Name
	networks := infra.Networks
	if len(networks) == 0 {
		networks = []string{fmt.Sprintf("yamlops-%s", g.env)}
	}

	networkConfigs := make(map[string]interface{})
	for _, net := range networks {
		networkConfigs[net] = map[string]interface{}{
			"aliases": []string{infra.Name},
		}
	}

	compose := map[string]interface{}{
		"services": map[string]interface{}{
			serviceName: map[string]interface{}{
				"image":          infra.Image,
				"container_name": serviceName,
				"restart":        "unless-stopped",
				"ports": []string{
					fmt.Sprintf("%d:%d", infra.GatewayPorts.HTTP, infra.GatewayPorts.HTTP),
					fmt.Sprintf("%d:%d", infra.GatewayPorts.HTTPS, infra.GatewayPorts.HTTPS),
				},
				"volumes": []string{
					"./gateway.yml:/app/configs/server.yml:ro",
					"./cache:/app/cache",
				},
				"extra_hosts": []string{
					"host.docker.internal:host-gateway",
				},
				"networks": networkConfigs,
			},
		},
		"networks": func() map[string]interface{} {
			nets := make(map[string]interface{})
			for _, net := range networks {
				nets[net] = map[string]interface{}{"external": true}
			}
			return nets
		}(),
	}

	data, err := yaml.Marshal(compose)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (g *Generator) generateInfraServiceSSL(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	if infra.SSLConfig == nil {
		return nil
	}

	serviceName := "yo-" + g.env + "-" + infra.Name
	networks := infra.Networks
	if len(networks) == 0 {
		networks = []string{fmt.Sprintf("yamlops-%s", g.env)}
	}

	networkConfigs := make(map[string]interface{})
	for _, net := range networks {
		networkConfigs[net] = map[string]interface{}{
			"aliases": []string{infra.Name},
		}
	}

	volumes := []string{
		"./ssl-data:/app/data",
		"./ssl-config:/app/configs:ro",
	}

	ports := []string{}
	if infra.SSLConfig.Ports.API > 0 {
		ports = append(ports, fmt.Sprintf("%d:%d", infra.SSLConfig.Ports.API, infra.SSLConfig.Ports.API))
	}

	compose := map[string]interface{}{
		"services": map[string]interface{}{
			serviceName: map[string]interface{}{
				"image":          infra.Image,
				"container_name": serviceName,
				"restart":        "unless-stopped",
				"ports":          ports,
				"volumes":        volumes,
				"networks":       networkConfigs,
			},
		},
		"networks": func() map[string]interface{} {
			nets := make(map[string]interface{})
			for _, net := range networks {
				nets[net] = map[string]interface{}{"external": true}
			}
			return nets
		}(),
	}

	data, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("failed to marshal ssl compose: %w", err)
	}

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", infra.Name))
	if err := os.WriteFile(composeFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write ssl compose file %s: %w", composeFile, err)
	}

	return nil
}
