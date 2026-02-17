package entity

import (
	"errors"
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type ServiceHealthcheck struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
}

func (h *ServiceHealthcheck) Validate() error {
	if h.Path == "" {
		return errors.New("healthcheck path is required")
	}
	if !strings.HasPrefix(h.Path, "/") {
		return errors.New("healthcheck path must start with /")
	}
	return nil
}

type ServiceResources struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type ServiceVolume struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Sync   bool   `yaml:"sync,omitempty"`
}

func (v *ServiceVolume) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var short string
	if err := unmarshal(&short); err == nil {
		parts := strings.SplitN(short, ":", 2)
		if len(parts) != 2 {
			return errors.New("invalid volume format, expected source:target")
		}
		v.Source = parts[0]
		v.Target = parts[1]
		return nil
	}

	type alias ServiceVolume
	var full alias
	if err := unmarshal(&full); err != nil {
		return err
	}
	v.Source = full.Source
	v.Target = full.Target
	v.Sync = full.Sync
	return nil
}

func (v *ServiceVolume) Validate() error {
	if v.Source == "" {
		return errors.New("volume source is required")
	}
	if v.Target == "" {
		return errors.New("volume target is required")
	}
	return nil
}

type ServicePort struct {
	Container int    `yaml:"container"`
	Host      int    `yaml:"host"`
	Protocol  string `yaml:"protocol,omitempty"`
}

func (p *ServicePort) Validate() error {
	if p.Container <= 0 || p.Container > 65535 {
		return fmt.Errorf("%w: container port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	if p.Host <= 0 || p.Host > 65535 {
		return fmt.Errorf("%w: host port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	if p.Protocol != "" && p.Protocol != "tcp" && p.Protocol != "udp" {
		return errors.New("protocol must be 'tcp' or 'udp'")
	}
	return nil
}

type ServiceGatewayRoute struct {
	Hostname      string `yaml:"hostname"`
	ContainerPort int    `yaml:"container_port"`
	Path          string `yaml:"path,omitempty"`
	HTTP          bool   `yaml:"http,omitempty"`
	HTTPS         bool   `yaml:"https,omitempty"`
}

func (r *ServiceGatewayRoute) Validate() error {
	if r.Hostname == "" {
		return errors.New("gateway hostname is required")
	}
	if r.ContainerPort <= 0 || r.ContainerPort > 65535 {
		return fmt.Errorf("%w: container_port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	return nil
}

func (r *ServiceGatewayRoute) HasGateway() bool {
	return r.HTTP || r.HTTPS
}

type Service struct {
	Name        string                           `yaml:"name"`
	Server      string                           `yaml:"server"`
	Image       string                           `yaml:"image"`
	Ports       []ServicePort                    `yaml:"ports,omitempty"`
	Env         map[string]valueobject.SecretRef `yaml:"env,omitempty"`
	Secrets     []string                         `yaml:"secrets,omitempty"`
	Healthcheck *ServiceHealthcheck              `yaml:"healthcheck,omitempty"`
	Resources   ServiceResources                 `yaml:"resources,omitempty"`
	Volumes     []ServiceVolume                  `yaml:"volumes,omitempty"`
	Gateways    []ServiceGatewayRoute            `yaml:"gateways,omitempty"`
	Internal    bool                             `yaml:"internal,omitempty"`
}

func (s *Service) GetServer() string {
	return s.Server
}

func (s *Service) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: service name is required", domain.ErrInvalidName)
	}
	if s.Server == "" {
		return errors.New("server is required")
	}
	if s.Image == "" {
		return errors.New("image is required")
	}
	for i, port := range s.Ports {
		if err := port.Validate(); err != nil {
			return fmt.Errorf("port %d: %w", i, err)
		}
	}
	if s.Healthcheck != nil {
		if err := s.Healthcheck.Validate(); err != nil {
			return err
		}
	}
	for i, vol := range s.Volumes {
		if err := vol.Validate(); err != nil {
			return fmt.Errorf("volume %d: %w", i, err)
		}
	}
	for i, gw := range s.Gateways {
		if err := gw.Validate(); err != nil {
			return fmt.Errorf("gateway %d: %w", i, err)
		}
	}
	return nil
}
