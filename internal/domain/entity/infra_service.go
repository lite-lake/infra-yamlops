package entity

import (
	"errors"
	"fmt"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type InfraServiceType string

const (
	InfraServiceTypeGateway InfraServiceType = "gateway"
	InfraServiceTypeSSL     InfraServiceType = "ssl"
)

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
			Ports  *SSLPorts  `yaml:"ports"`
			Config *SSLConfig `yaml:"config"`
		}
		if err := unmarshal(&ssl); err != nil {
			return err
		}
		s.SSLConfig = ssl.Config
		if s.SSLConfig == nil {
			s.SSLConfig = &SSLConfig{}
		}
		if ssl.Ports != nil {
			s.SSLConfig.Ports = *ssl.Ports
		}
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
			Config *SSLConfig       `yaml:"config,omitempty"`
		}{
			Name:   s.Name,
			Type:   s.Type,
			Server: s.Server,
			Image:  s.Image,
			Config: s.SSLConfig,
		}, nil
	}
	return (*infraServiceAlias)(s), nil
}

func (s *InfraService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: infra service name is required", domain.ErrInvalidName)
	}
	if s.Type != InfraServiceTypeGateway && s.Type != InfraServiceTypeSSL {
		return fmt.Errorf("invalid infra service type: %s", s.Type)
	}
	if s.Server == "" {
		return errors.New("server is required")
	}
	if s.Image == "" {
		return errors.New("image is required")
	}
	switch s.Type {
	case InfraServiceTypeGateway:
		if s.GatewayConfig == nil {
			return errors.New("gateway config is required for gateway type")
		}
	case InfraServiceTypeSSL:
		if s.SSLConfig == nil {
			return errors.New("ssl config is required for ssl type")
		}
		if err := s.SSLConfig.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type SSLConfig struct {
	Ports    SSLPorts    `yaml:"ports,omitempty"`
	Auth     SSLAuth     `yaml:"auth"`
	Storage  SSLStorage  `yaml:"storage"`
	Defaults SSLDefaults `yaml:"defaults"`
}

func (c *SSLConfig) Validate() error {
	if err := c.Ports.Validate(); err != nil {
		return err
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}
	if err := c.Storage.Validate(); err != nil {
		return err
	}
	if err := c.Defaults.Validate(); err != nil {
		return err
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

type SSLAuth struct {
	Enabled bool                  `yaml:"enabled"`
	APIKey  valueobject.SecretRef `yaml:"apikey"`
}

func (a *SSLAuth) Validate() error {
	if a.Enabled {
		if err := a.APIKey.Validate(); err != nil {
			return fmt.Errorf("apikey: %w", err)
		}
	}
	return nil
}

type SSLStorage struct {
	Type string `yaml:"type"`
	Path string `yaml:"path,omitempty"`
}

func (s *SSLStorage) Validate() error {
	if s.Type == "" {
		return errors.New("storage type is required")
	}
	return nil
}

type SSLDefaults struct {
	IssueProvider   string `yaml:"issue_provider"`
	StorageProvider string `yaml:"storage_provider"`
}

func (d *SSLDefaults) Validate() error {
	if d.IssueProvider == "" {
		return errors.New("issue_provider is required")
	}
	if d.StorageProvider == "" {
		return errors.New("storage_provider is required")
	}
	return nil
}
