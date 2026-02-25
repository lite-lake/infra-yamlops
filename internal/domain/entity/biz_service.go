package entity

import (
	"fmt"
	"sort"
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
		return domain.RequiredField("healthcheck path")
	}
	if !strings.HasPrefix(h.Path, "/") {
		return fmt.Errorf("%w: healthcheck path must start with /", domain.ErrInvalidDomain)
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
			return fmt.Errorf("%w: invalid volume format, expected source:target", domain.ErrInvalidDomain)
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
		return domain.RequiredField("volume source")
	}
	if v.Target == "" {
		return domain.RequiredField("volume target")
	}
	return nil
}

type ServicePort struct {
	Container int    `yaml:"container"`
	Host      int    `yaml:"host"`
	Protocol  string `yaml:"protocol,omitempty"`
}

func (p *ServicePort) Validate() error {
	if p.Container <= 0 || p.Container > domain.MaxPortNumber {
		return fmt.Errorf("%w: container port must be between 1 and %d", domain.ErrInvalidPort, domain.MaxPortNumber)
	}
	if p.Host <= 0 || p.Host > domain.MaxPortNumber {
		return fmt.Errorf("%w: host port must be between 1 and %d", domain.ErrInvalidPort, domain.MaxPortNumber)
	}
	if p.Protocol != "" && p.Protocol != "tcp" && p.Protocol != "udp" {
		return fmt.Errorf("%w: protocol must be 'tcp' or 'udp'", domain.ErrInvalidProtocol)
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
		return domain.RequiredField("gateway hostname")
	}
	if r.ContainerPort <= 0 || r.ContainerPort > domain.MaxPortNumber {
		return fmt.Errorf("%w: container_port must be between 1 and %d", domain.ErrInvalidPort, domain.MaxPortNumber)
	}
	return nil
}

func (r *ServiceGatewayRoute) HasGateway() bool {
	return r.HTTP || r.HTTPS
}

type BizService struct {
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
	Networks    []string                         `yaml:"networks,omitempty"`
}

func (s *BizService) GetServer() string {
	return s.Server
}

func (s *BizService) GetNetworks() []string {
	return s.Networks
}

func (s *BizService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: service name is required", domain.ErrInvalidName)
	}
	if s.Server == "" {
		return domain.RequiredField("server")
	}
	if s.Image == "" {
		return domain.RequiredField("image")
	}
	for i, port := range s.Ports {
		if err := port.Validate(); err != nil {
			return fmt.Errorf("port %d: %w", i, err)
		}
	}
	envKeys := make([]string, 0, len(s.Env))
	for key := range s.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		ref := s.Env[key]
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("env[%s]: %w", key, err)
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
