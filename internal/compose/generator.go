package compose

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type HealthCheck struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

type ResourceLimits struct {
	Cpus   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type Resources struct {
	Limits       *ResourceLimits `yaml:"limits,omitempty"`
	Reservations *ResourceLimits `yaml:"reservations,omitempty"`
}

type Deploy struct {
	Resources *Resources `yaml:"resources,omitempty"`
}

type Service struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	HealthCheck   *HealthCheck      `yaml:"healthcheck,omitempty"`
	Deploy        *Deploy           `yaml:"deploy,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
}

type ComposeFile struct {
	Version  string              `yaml:"version"`
	Services map[string]Service  `yaml:"services"`
	Networks map[string]struct{} `yaml:"networks,omitempty"`
}

type ComposeService struct {
	Name        string
	Image       string
	Ports       []string
	Environment map[string]string
	Volumes     []string
	HealthCheck *HealthCheck
	Resources   *Resources
	Internal    bool
}

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(svc *ComposeService) (string, error) {
	if svc == nil {
		return "", fmt.Errorf("service cannot be nil")
	}
	if svc.Name == "" {
		return "", fmt.Errorf("service name cannot be empty")
	}
	if svc.Image == "" {
		return "", fmt.Errorf("service image cannot be empty")
	}

	serviceName := "yo-" + svc.Name

	service := Service{
		Image:         svc.Image,
		ContainerName: serviceName,
		Ports:         svc.Ports,
		Environment:   svc.Environment,
		Volumes:       svc.Volumes,
		HealthCheck:   svc.HealthCheck,
		Networks:      []string{"yamlops"},
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
			"yamlops": {},
		},
	}

	data, err := yaml.Marshal(&compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose file: %w", err)
	}

	return string(data), nil
}
