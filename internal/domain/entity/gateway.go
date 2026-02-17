package entity

import (
	"errors"
	"fmt"
	"net"

	"github.com/litelake/yamlops/internal/domain"
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
		return errors.New("ssl mode must be 'local' or 'remote'")
	}
	if s.Mode == "remote" && s.Endpoint == "" {
		return errors.New("endpoint is required for remote ssl mode")
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
			return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
		}
	}
	return nil
}

type GatewayConfig struct {
	Source string `yaml:"source"`
	Sync   bool   `yaml:"sync"`
}

type Gateway struct {
	Name     string           `yaml:"name"`
	Zone     string           `yaml:"zone"`
	Server   string           `yaml:"server"`
	Image    string           `yaml:"image"`
	Ports    GatewayPorts     `yaml:"ports"`
	Config   GatewayConfig    `yaml:"config"`
	SSL      GatewaySSLConfig `yaml:"ssl"`
	WAF      GatewayWAFConfig `yaml:"waf"`
	LogLevel int              `yaml:"log_level,omitempty"`
}

func (g *Gateway) GetServer() string {
	return g.Server
}

func (g *Gateway) Validate() error {
	if g.Name == "" {
		return fmt.Errorf("%w: gateway name is required", domain.ErrInvalidName)
	}
	if g.Zone == "" {
		return errors.New("zone is required")
	}
	if g.Server == "" {
		return errors.New("server is required")
	}
	if g.Image == "" {
		return errors.New("image is required")
	}
	if err := g.Ports.Validate(); err != nil {
		return err
	}
	if err := g.SSL.Validate(); err != nil {
		return err
	}
	if err := g.WAF.Validate(); err != nil {
		return err
	}
	return nil
}
