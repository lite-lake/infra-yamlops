package entity

import (
	"fmt"
	"net"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type NetworkType string

const (
	NetworkTypeBridge  NetworkType = "bridge"
	NetworkTypeOverlay NetworkType = "overlay"
)

type ServerNetwork struct {
	Name   string      `yaml:"name"`
	Type   NetworkType `yaml:"type,omitempty"`
	Driver string      `yaml:"driver,omitempty"`
}

func (n *ServerNetwork) Validate() error {
	if n.Name == "" {
		return domain.RequiredField("network name")
	}
	if n.Type != "" && n.Type != NetworkTypeBridge && n.Type != NetworkTypeOverlay {
		return fmt.Errorf("%w: network type must be 'bridge' or 'overlay'", domain.ErrInvalidType)
	}
	return nil
}

func (n *ServerNetwork) GetType() NetworkType {
	if n.Type == "" {
		return NetworkTypeBridge
	}
	return n.Type
}

func (n *ServerNetwork) GetDriver() string {
	if n.Driver != "" {
		return n.Driver
	}
	return string(n.GetType())
}

type ServerIP struct {
	Public  string `yaml:"public,omitempty"`
	Private string `yaml:"private,omitempty"`
}

func (i *ServerIP) Validate() error {
	if i.Public != "" && net.ParseIP(i.Public) == nil {
		return fmt.Errorf("%w: public IP %s", domain.ErrInvalidIP, i.Public)
	}
	if i.Private != "" && net.ParseIP(i.Private) == nil {
		return fmt.Errorf("%w: private IP %s", domain.ErrInvalidIP, i.Private)
	}
	return nil
}

type ServerSSH struct {
	Host     string                `yaml:"host"`
	Port     int                   `yaml:"port"`
	User     string                `yaml:"user"`
	Password valueobject.SecretRef `yaml:"password"`
}

func (s *ServerSSH) Validate() error {
	if s.Host == "" {
		return domain.RequiredField("ssh host")
	}
	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("%w: ssh port must be between 1 and 65535", domain.ErrInvalidPort)
	}
	if s.User == "" {
		return domain.RequiredField("ssh user")
	}
	if err := s.Password.Validate(); err != nil {
		return fmt.Errorf("ssh password: %w", err)
	}
	return nil
}

type ServerEnvironment struct {
	APTSource  string   `yaml:"apt_source,omitempty"`
	Registries []string `yaml:"registries,omitempty"`
}

type Server struct {
	Name        string            `yaml:"name"`
	Zone        string            `yaml:"zone"`
	ISP         string            `yaml:"isp,omitempty"`
	OS          string            `yaml:"os"`
	IP          ServerIP          `yaml:"ip"`
	SSH         ServerSSH         `yaml:"ssh"`
	Environment ServerEnvironment `yaml:"environment,omitempty"`
	Networks    []ServerNetwork   `yaml:"networks,omitempty"`
}

func (s *Server) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: server name is required", domain.ErrInvalidName)
	}
	if s.Zone == "" {
		return domain.RequiredField("zone")
	}
	if err := s.IP.Validate(); err != nil {
		return err
	}
	if err := s.SSH.Validate(); err != nil {
		return err
	}
	for i, net := range s.Networks {
		if err := net.Validate(); err != nil {
			return fmt.Errorf("network %d: %w", i, err)
		}
	}
	return nil
}

func (s *Server) GetNetworkNames() []string {
	if len(s.Networks) == 0 {
		return nil
	}
	names := make([]string, len(s.Networks))
	for i, n := range s.Networks {
		names[i] = n.Name
	}
	return names
}

func (s *Server) HasNetwork(name string) bool {
	for _, n := range s.Networks {
		if n.Name == name {
			return true
		}
	}
	return false
}

func (s *Server) GetNetwork(name string) *ServerNetwork {
	for i := range s.Networks {
		if s.Networks[i].Name == name {
			return &s.Networks[i]
		}
	}
	return nil
}
