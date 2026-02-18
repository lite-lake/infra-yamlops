package entity

import (
	"fmt"
	"strconv"

	"github.com/litelake/yamlops/internal/domain"
)

type Config struct {
	Secrets       []Secret       `yaml:"secrets,omitempty"`
	ISPs          []ISP          `yaml:"isps,omitempty"`
	Registries    []Registry     `yaml:"registries,omitempty"`
	Zones         []Zone         `yaml:"zones,omitempty"`
	Servers       []Server       `yaml:"servers,omitempty"`
	InfraServices []InfraService `yaml:"infra_services,omitempty"`
	Gateways      []Gateway      `yaml:"gateways,omitempty"`
	Services      []BizService   `yaml:"services,omitempty"`
	Domains       []Domain       `yaml:"domains,omitempty"`
	Certificates  []Certificate  `yaml:"certificates,omitempty"`
}

func (c *Config) Validate() error {
	for i, s := range c.Secrets {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("secrets[%d]: %w", i, err)
		}
	}
	for i, isp := range c.ISPs {
		if err := isp.Validate(); err != nil {
			return fmt.Errorf("isps[%d]: %w", i, err)
		}
	}
	for i, r := range c.Registries {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("registries[%d]: %w", i, err)
		}
	}
	for i, z := range c.Zones {
		if err := z.Validate(); err != nil {
			return fmt.Errorf("zones[%d]: %w", i, err)
		}
	}
	for i, s := range c.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("servers[%d]: %w", i, err)
		}
	}
	for i, infra := range c.InfraServices {
		if err := infra.Validate(); err != nil {
			return fmt.Errorf("infra_services[%d]: %w", i, err)
		}
	}
	for i, g := range c.Gateways {
		if err := g.Validate(); err != nil {
			return fmt.Errorf("gateways[%d]: %w", i, err)
		}
	}
	for i, s := range c.Services {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("services[%d]: %w", i, err)
		}
	}
	for i, d := range c.Domains {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("domains[%d]: %w", i, err)
		}
	}
	for i, cert := range c.Certificates {
		if err := cert.Validate(); err != nil {
			return fmt.Errorf("certificates[%d]: %w", i, err)
		}
	}
	return nil
}

func (c *Config) GetSecretsMap() map[string]string {
	m := make(map[string]string)
	for _, s := range c.Secrets {
		m[s.Name] = s.Value
	}
	return m
}

func (c *Config) GetISPMap() map[string]*ISP {
	m := make(map[string]*ISP)
	for i := range c.ISPs {
		m[c.ISPs[i].Name] = &c.ISPs[i]
	}
	return m
}

func (c *Config) GetZoneMap() map[string]*Zone {
	m := make(map[string]*Zone)
	for i := range c.Zones {
		m[c.Zones[i].Name] = &c.Zones[i]
	}
	return m
}

func (c *Config) GetInfraServiceMap() map[string]*InfraService {
	m := make(map[string]*InfraService)
	for i := range c.InfraServices {
		m[c.InfraServices[i].Name] = &c.InfraServices[i]
	}
	return m
}

func (c *Config) GetGatewayMap() map[string]*Gateway {
	m := make(map[string]*Gateway)
	for i := range c.Gateways {
		m[c.Gateways[i].Name] = &c.Gateways[i]
	}
	return m
}

func (c *Config) GetServerMap() map[string]*Server {
	m := make(map[string]*Server)
	for i := range c.Servers {
		m[c.Servers[i].Name] = &c.Servers[i]
	}
	return m
}

func (c *Config) GetServiceMap() map[string]*BizService {
	m := make(map[string]*BizService)
	for i := range c.Services {
		m[c.Services[i].Name] = &c.Services[i]
	}
	return m
}

func (c *Config) GetRegistryMap() map[string]*Registry {
	m := make(map[string]*Registry)
	for i := range c.Registries {
		m[c.Registries[i].Name] = &c.Registries[i]
	}
	return m
}

func (c *Config) GetDomainMap() map[string]*Domain {
	m := make(map[string]*Domain)
	for i := range c.Domains {
		m[c.Domains[i].Name] = &c.Domains[i]
	}
	return m
}

func (c *Config) GetAllDNSRecords() []DNSRecord {
	var records []DNSRecord
	for _, d := range c.Domains {
		records = append(records, d.FlattenRecords()...)
	}
	return records
}

func (c *Config) GetCertificateMap() map[string]*Certificate {
	m := make(map[string]*Certificate)
	for i := range c.Certificates {
		m[c.Certificates[i].Name] = &c.Certificates[i]
	}
	return m
}

func ParsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", domain.ErrInvalidPort, s)
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("%w: %d", domain.ErrInvalidPort, port)
	}
	return port, nil
}
