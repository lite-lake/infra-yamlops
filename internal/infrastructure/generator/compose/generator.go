package compose

import (
	"fmt"

	"github.com/litelake/yamlops/internal/constants"
	domainerr "github.com/litelake/yamlops/internal/domain"
	"gopkg.in/yaml.v3"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(svc *ComposeService, env string) (string, error) {
	if svc == nil {
		return "", fmt.Errorf("%w: service cannot be nil", domainerr.ErrServiceInvalid)
	}
	if svc.Name == "" {
		return "", fmt.Errorf("%w: service name cannot be empty", domainerr.ErrRequired)
	}
	if svc.Image == "" {
		return "", fmt.Errorf("%w: service image cannot be empty", domainerr.ErrRequired)
	}
	if env == "" {
		env = "dev"
	}

	serviceName := "yo-" + env + "-" + svc.Name

	networkConfigs := make(map[string]*NetworkConfig)
	for _, netName := range svc.Networks {
		networkConfigs[netName] = &NetworkConfig{
			Aliases: []string{svc.Name},
		}
	}

	service := Service{
		Image:         svc.Image,
		ContainerName: serviceName,
		Ports:         svc.Ports,
		Environment:   svc.Environment,
		EnvFiles:      svc.EnvFiles,
		Volumes:       svc.Volumes,
		HealthCheck:   svc.HealthCheck,
		Networks:      networkConfigs,
		Restart:       constants.DefaultRestartPolicy,
		ExtraHosts:    svc.ExtraHosts,
	}

	if svc.Resources != nil {
		service.Deploy = &Deploy{
			Resources: svc.Resources,
		}
	}

	networks := make(map[string]*ExternalNetwork)
	for _, netName := range svc.Networks {
		networks[netName] = &ExternalNetwork{External: true}
	}

	compose := ComposeFile{
		Version: constants.ComposeVersion,
		Services: map[string]Service{
			serviceName: service,
		},
		Networks: networks,
	}

	data, err := yaml.Marshal(&compose)
	if err != nil {
		return "", fmt.Errorf("%w: %w", domainerr.ErrComposeGenerateFailed, err)
	}

	return string(data), nil
}
