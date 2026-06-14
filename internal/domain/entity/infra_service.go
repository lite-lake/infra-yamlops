package entity

import (
	"fmt"
	"net"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type InfraServiceType string

const (
	InfraServiceTypeGateway InfraServiceType = "gateway"
)

type GatewayPorts struct {
	HTTP  int `yaml:"http"`
	HTTPS int `yaml:"https"`
}

func (p *GatewayPorts) Validate() error {
	if err := ValidatePort(p.HTTP); err != nil {
		return fmt.Errorf("http %w", err)
	}
	if err := ValidatePort(p.HTTPS); err != nil {
		return fmt.Errorf("https %w", err)
	}
	return nil
}

type GatewaySSLConfig struct {
	Mode     string `yaml:"mode"`
	Endpoint string `yaml:"endpoint,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
}

func (s *GatewaySSLConfig) Validate() error {
	if s.Mode != "local" && s.Mode != "remote" {
		return fmt.Errorf("%w: ssl mode must be 'local' or 'remote'", domain.ErrInvalidType)
	}
	if s.Mode == "remote" && s.Endpoint == "" {
		return domain.RequiredField("endpoint for remote ssl mode")
	}
	return nil
}

type GatewayWAFConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Whitelist []string `yaml:"whitelist,omitempty"`
}

func (w *GatewayWAFConfig) Validate() error {
	for _, cidr := range w.Whitelist {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("%w: %s", domain.ErrInvalidCIDR, cidr)
		}
	}
	return nil
}

type GatewayNotification struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout,omitempty"` // 默认 "5s"
}

func (n *GatewayNotification) Validate() error {
	if n.Enabled && n.URL == "" {
		return domain.RequiredField("notification url when enabled")
	}
	return nil
}

type InfraService struct {
	ServiceBase
	Name  string           `yaml:"name"`
	Type  InfraServiceType `yaml:"type"`
	Image string           `yaml:"image"`

	Secrets []string                         `yaml:"secrets,omitempty"`
	Env     map[string]valueobject.SecretRef `yaml:"env,omitempty"`

	GatewayPorts        *GatewayPorts        `yaml:"ports,omitempty"`
	GatewaySSL          *GatewaySSLConfig    `yaml:"ssl,omitempty"`
	GatewayWAF          *GatewayWAFConfig    `yaml:"waf,omitempty"`
	GatewayLogLevel     int                  `yaml:"log_level,omitempty"`
	GatewayNotification *GatewayNotification `yaml:"notification,omitempty"`

	Ports []ServicePort `yaml:"extra_ports,omitempty"`
}

type infraServiceAlias InfraService

func (s *InfraService) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Name       string                           `yaml:"name"`
		Type       InfraServiceType                 `yaml:"type"`
		Server     string                           `yaml:"server"`
		Image      string                           `yaml:"image"`
		Networks   []string                         `yaml:"networks"`
		Secrets    []string                         `yaml:"secrets"`
		Env        map[string]valueobject.SecretRef `yaml:"env"`
		ExtraPorts []ServicePort                    `yaml:"extra_ports"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	s.Name = raw.Name
	s.Type = raw.Type
	s.ServiceBase.Server = raw.Server
	s.ServiceBase.Networks = raw.Networks
	s.Image = raw.Image
	s.Secrets = raw.Secrets
	s.Env = raw.Env
	s.Ports = raw.ExtraPorts

	switch s.Type {
	case InfraServiceTypeGateway:
		var gw struct {
			Ports        *GatewayPorts        `yaml:"ports"`
			SSL          *GatewaySSLConfig    `yaml:"ssl"`
			WAF          *GatewayWAFConfig    `yaml:"waf"`
			LogLevel     int                  `yaml:"log_level"`
			Notification *GatewayNotification `yaml:"notification"`
		}
		if err := unmarshal(&gw); err != nil {
			return err
		}
		s.GatewayPorts = gw.Ports
		s.GatewaySSL = gw.SSL
		s.GatewayWAF = gw.WAF
		s.GatewayLogLevel = gw.LogLevel
		s.GatewayNotification = gw.Notification

	}

	return nil
}

func (s *InfraService) MarshalYAML() (interface{}, error) {
	switch s.Type {
	case InfraServiceTypeGateway:
		return struct {
			Name         string                           `yaml:"name"`
			Type         InfraServiceType                 `yaml:"type"`
			Server       string                           `yaml:"server"`
			Image        string                           `yaml:"image"`
			Secrets      []string                         `yaml:"secrets,omitempty"`
			Env          map[string]valueobject.SecretRef `yaml:"env,omitempty"`
			Ports        *GatewayPorts                    `yaml:"ports,omitempty"`
			SSL          *GatewaySSLConfig                `yaml:"ssl,omitempty"`
			WAF          *GatewayWAFConfig                `yaml:"waf,omitempty"`
			LogLevel     int                              `yaml:"log_level,omitempty"`
			Notification *GatewayNotification             `yaml:"notification,omitempty"`
			Networks     []string                         `yaml:"networks,omitempty"`
			ExtraPorts   []ServicePort                    `yaml:"extra_ports,omitempty"`
		}{
			Name:         s.Name,
			Type:         s.Type,
			Server:       s.ServiceBase.Server,
			Image:        s.Image,
			Secrets:      s.Secrets,
			Env:          s.Env,
			Ports:        s.GatewayPorts,
			SSL:          s.GatewaySSL,
			WAF:          s.GatewayWAF,
			LogLevel:     s.GatewayLogLevel,
			Notification: s.GatewayNotification,
			Networks:     s.ServiceBase.Networks,
			ExtraPorts:   s.Ports,
		}, nil
	}
	return (*infraServiceAlias)(s), nil
}

func (s *InfraService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: infra service name is required", domain.ErrInvalidName)
	}
	if s.Type != InfraServiceTypeGateway {
		return fmt.Errorf("%w: %s", domain.ErrInvalidType, s.Type)
	}
	if s.Server == "" {
		return domain.RequiredField("server")
	}
	if s.Image == "" {
		return domain.RequiredField("image")
	}
	switch s.Type {
	case InfraServiceTypeGateway:
		if s.GatewayPorts != nil {
			if err := s.GatewayPorts.Validate(); err != nil {
				return err
			}
		}
		if s.GatewayNotification != nil {
			if err := s.GatewayNotification.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}
