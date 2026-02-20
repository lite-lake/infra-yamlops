package entity

import (
	"fmt"
	"net"

	"github.com/litelake/yamlops/internal/domain"
)

type InfraServiceType string

const (
	InfraServiceTypeGateway InfraServiceType = "gateway"
	InfraServiceTypeSSL     InfraServiceType = "ssl"
)

type GatewayPorts struct {
	HTTP  int `yaml:"http"`
	HTTPS int `yaml:"https"`
}

func (p *GatewayPorts) Validate() error {
	if p.HTTP <= 0 || p.HTTP > 65535 {
		return fmt.Errorf("%w: http port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	if p.HTTPS <= 0 || p.HTTPS > 65535 {
		return fmt.Errorf("%w: https port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	return nil
}

type GatewaySSLConfig struct {
	Mode     string `yaml:"mode"`
	Endpoint string `yaml:"endpoint,omitempty"`
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

type GatewayConfig struct {
	Source string `yaml:"source"`
	Sync   bool   `yaml:"sync"`
}

type InfraService struct {
	Name   string           `yaml:"name"`
	Type   InfraServiceType `yaml:"type"`
	Server string           `yaml:"server"`
	Image  string           `yaml:"image"`

	GatewayPorts    *GatewayPorts     `yaml:"ports,omitempty"`
	GatewayConfig   *GatewayConfig    `yaml:"config,omitempty"`
	GatewaySSL      *GatewaySSLConfig `yaml:"ssl,omitempty"`
	GatewayWAF      *GatewayWAFConfig `yaml:"waf,omitempty"`
	GatewayLogLevel int               `yaml:"log_level,omitempty"`

	SSLConfig *SSLConfig `yaml:"-"`
}

type infraServiceAlias InfraService

func (s *InfraService) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Name   string           `yaml:"name"`
		Type   InfraServiceType `yaml:"type"`
		Server string           `yaml:"server"`
		Image  string           `yaml:"image"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	s.Name = raw.Name
	s.Type = raw.Type
	s.Server = raw.Server
	s.Image = raw.Image

	switch s.Type {
	case InfraServiceTypeGateway:
		var gw struct {
			Ports    *GatewayPorts     `yaml:"ports"`
			Config   *GatewayConfig    `yaml:"config"`
			SSL      *GatewaySSLConfig `yaml:"ssl"`
			WAF      *GatewayWAFConfig `yaml:"waf"`
			LogLevel int               `yaml:"log_level"`
		}
		if err := unmarshal(&gw); err != nil {
			return err
		}
		s.GatewayPorts = gw.Ports
		s.GatewayConfig = gw.Config
		s.GatewaySSL = gw.SSL
		s.GatewayWAF = gw.WAF
		s.GatewayLogLevel = gw.LogLevel

	case InfraServiceTypeSSL:
		var ssl struct {
			Ports  *SSLPorts        `yaml:"ports"`
			Config *SSLVolumeConfig `yaml:"config"`
		}
		if err := unmarshal(&ssl); err != nil {
			return err
		}
		s.SSLConfig = &SSLConfig{}
		if ssl.Ports != nil {
			s.SSLConfig.Ports = *ssl.Ports
		}
		s.SSLConfig.Config = ssl.Config
	}

	return nil
}

func (s *InfraService) MarshalYAML() (interface{}, error) {
	switch s.Type {
	case InfraServiceTypeGateway:
		return struct {
			Name     string            `yaml:"name"`
			Type     InfraServiceType  `yaml:"type"`
			Server   string            `yaml:"server"`
			Image    string            `yaml:"image"`
			Ports    *GatewayPorts     `yaml:"ports,omitempty"`
			Config   *GatewayConfig    `yaml:"config,omitempty"`
			SSL      *GatewaySSLConfig `yaml:"ssl,omitempty"`
			WAF      *GatewayWAFConfig `yaml:"waf,omitempty"`
			LogLevel int               `yaml:"log_level,omitempty"`
		}{
			Name:     s.Name,
			Type:     s.Type,
			Server:   s.Server,
			Image:    s.Image,
			Ports:    s.GatewayPorts,
			Config:   s.GatewayConfig,
			SSL:      s.GatewaySSL,
			WAF:      s.GatewayWAF,
			LogLevel: s.GatewayLogLevel,
		}, nil
	case InfraServiceTypeSSL:
		return struct {
			Name   string           `yaml:"name"`
			Type   InfraServiceType `yaml:"type"`
			Server string           `yaml:"server"`
			Image  string           `yaml:"image"`
			Ports  *SSLPorts        `yaml:"ports,omitempty"`
			Config *SSLVolumeConfig `yaml:"config,omitempty"`
		}{
			Name:   s.Name,
			Type:   s.Type,
			Server: s.Server,
			Image:  s.Image,
			Ports:  &s.SSLConfig.Ports,
			Config: s.SSLConfig.Config,
		}, nil
	}
	return (*infraServiceAlias)(s), nil
}

func (s *InfraService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: infra service name is required", domain.ErrInvalidName)
	}
	if s.Type != InfraServiceTypeGateway && s.Type != InfraServiceTypeSSL {
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
		if s.GatewayConfig == nil {
			return domain.RequiredField("gateway config for gateway type")
		}
		if s.GatewayPorts != nil {
			if err := s.GatewayPorts.Validate(); err != nil {
				return err
			}
		}
	case InfraServiceTypeSSL:
		if s.SSLConfig == nil {
			return domain.RequiredField("ssl config for ssl type")
		}
		if err := s.SSLConfig.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s *InfraService) GetServer() string {
	return s.Server
}

type SSLVolumeConfig struct {
	Source string `yaml:"source"`
	Sync   bool   `yaml:"sync"`
}

type SSLConfig struct {
	Ports  SSLPorts         `yaml:"ports,omitempty"`
	Config *SSLVolumeConfig `yaml:"config,omitempty"`
}

func (c *SSLConfig) Validate() error {
	if err := c.Ports.Validate(); err != nil {
		return err
	}
	if c.Config == nil {
		return domain.RequiredField("config for ssl type")
	}
	if c.Config.Source == "" {
		return domain.RequiredField("config.source for ssl type")
	}
	return nil
}

type SSLPorts struct {
	API int `yaml:"api"`
}

func (p *SSLPorts) Validate() error {
	if p.API <= 0 || p.API > 65535 {
		return fmt.Errorf("%w: api port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	return nil
}
