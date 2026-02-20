package entity

import (
	"fmt"
	"net"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

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
	return nil
}
