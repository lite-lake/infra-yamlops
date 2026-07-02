package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/application/generator/compose"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func (g *Generator) generateInfraServiceComposes(config *entity.Config) error {
	serverInfraServices := make(map[string][]*entity.InfraService)
	for i := range config.InfraServices {
		infra := &config.InfraServices[i]
		serverInfraServices[infra.Server] = append(serverInfraServices[infra.Server], infra)
	}

	for serverName, infraServices := range serverInfraServices {
		serverDir := filepath.Join(g.outputDir, g.env, serverName)
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

func (g *Generator) generateInfraServiceComposesWithScope(config *entity.Config, scope *valueobject.Scope) error {
	serverInfraServices := make(map[string][]*entity.InfraService)
	for i := range config.InfraServices {
		infra := &config.InfraServices[i]
		if scope.ShouldGenerateInfraService(infra.Name, infra.Server) {
			serverInfraServices[infra.Server] = append(serverInfraServices[infra.Server], infra)
		}
	}

	for serverName, infraServices := range serverInfraServices {
		serverDir := filepath.Join(g.outputDir, g.env, serverName)
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
	}
	return nil
}

func (g *Generator) generateInfraServiceGateway(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	if infra.GatewayPorts != nil {
		composeContent, err := g.generateInfraGatewayCompose(serverDir, infra, config)
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

func (g *Generator) generateInfraGatewayCompose(serverDir string, infra *entity.InfraService, config *entity.Config) (string, error) {
	ports := []string{
		fmt.Sprintf("%d:%d", infra.GatewayPorts.HTTP, infra.GatewayPorts.HTTP),
		fmt.Sprintf("%d:%d", infra.GatewayPorts.HTTPS, infra.GatewayPorts.HTTPS),
	}
	for _, p := range infra.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d", p.Host, p.Container))
	}

	volumes := []string{
		constants.GatewayConfigPath,
		constants.GatewayCachePath,
	}

	networks := infra.Networks
	if len(networks) == 0 {
		networks = []string{fmt.Sprintf("yamlops-%s", g.env)}
	}

	envMap := make(map[string]string)
	secrets := config.GetSecretsMap()
	for k, ref := range infra.Env {
		val, err := ref.Resolve(secrets)
		if err != nil {
			return "", fmt.Errorf("failed to resolve env %s for gateway %s: %w", k, infra.Name, err)
		}
		envMap[k] = val
	}
	for _, secretName := range infra.Secrets {
		if val, ok := secrets[secretName]; ok {
			envMap[strings.ToUpper(secretName)] = val
		}
	}

	serverMap := config.GetServerMap()
	if server, ok := serverMap[infra.Server]; ok {
		envMap["DEPLOY_ZONE_NAME"] = server.Zone
	}
	envMap["DEPLOY_ENV_NAME"] = g.env
	envMap["DEPLOY_SERVER_NAME"] = infra.Server
	envMap["DEPLOY_SERVICE_NAME"] = infra.Name

	envFileName := fmt.Sprintf("%s.env", infra.Name)
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
		Name:       infra.Name,
		Image:      infra.Image,
		Ports:      ports,
		EnvFiles:   []string{envFileName},
		Volumes:    volumes,
		Networks:   networks,
		ExtraHosts: []string{constants.HostDockerGateway},
	}

	return g.composeGen.Generate(composeSvc, g.env)
}
