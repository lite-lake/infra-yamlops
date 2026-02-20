package compose

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(svc *ComposeService, env string) (string, error) {
	if svc == nil {
		return "", fmt.Errorf("service cannot be nil")
	}
	if svc.Name == "" {
		return "", fmt.Errorf("service name cannot be empty")
	}
	if svc.Image == "" {
		return "", fmt.Errorf("service image cannot be empty")
	}
	if env == "" {
		env = "dev"
	}

	serviceName := "yo-" + env + "-" + svc.Name
	networkName := "yamlops-" + env

	service := Service{
		Image:         svc.Image,
		ContainerName: serviceName,
		Ports:         svc.Ports,
		Environment:   svc.Environment,
		Volumes:       svc.Volumes,
		HealthCheck:   svc.HealthCheck,
		Networks:      []string{networkName},
		Restart:       "unless-stopped",
	}

	if svc.Resources != nil {
		service.Deploy = &Deploy{
			Resources: svc.Resources,
		}
	}

	compose := ComposeFile{
		Version: "3.8",
		Services: map[string]Service{
			serviceName: service,
		},
		Networks: map[string]struct{}{
			networkName: {},
		},
	}

	data, err := yaml.Marshal(&compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose file: %w", err)
	}

	return string(data), nil
}
