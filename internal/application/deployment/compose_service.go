package deployment

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/infrastructure/generator/compose"
)

func (g *Generator) generateServiceComposes(config *entity.Config) error {
	serverServices := make(map[string][]*entity.BizService)
	for i := range config.Services {
		svc := &config.Services[i]
		serverServices[svc.Server] = append(serverServices[svc.Server], svc)
	}

	for serverName, services := range serverServices {
		serverDir := filepath.Join(g.outputDir, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverName, err)
		}

		for _, svc := range services {
			if err := g.generateServiceCompose(serverDir, svc, config.GetSecretsMap()); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Generator) generateServiceCompose(serverDir string, svc *entity.BizService, secrets map[string]string) error {
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
