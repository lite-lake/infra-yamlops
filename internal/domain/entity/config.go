package entity

import (
	"fmt"
	"strconv"

	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain"
)

type Config struct {
	Secrets       []Secret       `yaml:"secrets,omitempty"`
	ISPs          []ISP          `yaml:"isps,omitempty"`
	Registries    []Registry     `yaml:"registries,omitempty"`
	Zones         []Zone         `yaml:"zones,omitempty"`
	Servers       []Server       `yaml:"servers,omitempty"`
	InfraServices []InfraService `yaml:"infra_services,omitempty"`
	Services      []BizService   `yaml:"services,omitempty"`
	Domains       []Domain       `yaml:"domains,omitempty"`
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
	return nil
}

func toMapPtr[T any](items []T, getName func(T) string) map[string]*T {
	m := make(map[string]*T)
	for i := range items {
		m[getName(items[i])] = &items[i]
	}
	return m
}

func (c *Config) GetSecretsMap() map[string]string {
	m := make(map[string]string)
	for _, s := range c.Secrets {
		m[s.Name] = s.Value
	}
	return m
}

func (c *Config) GetISPMap() map[string]*ISP {
	return toMapPtr(c.ISPs, func(isp ISP) string { return isp.Name })
}

func (c *Config) GetZoneMap() map[string]*Zone {
	return toMapPtr(c.Zones, func(z Zone) string { return z.Name })
}

func (c *Config) GetInfraServiceMap() map[string]*InfraService {
	return toMapPtr(c.InfraServices, func(s InfraService) string { return s.Name })
}

func (c *Config) GetServerMap() map[string]*Server {
	return toMapPtr(c.Servers, func(s Server) string { return s.Name })
}

func (c *Config) GetServiceMap() map[string]*BizService {
	return toMapPtr(c.Services, func(s BizService) string { return s.Name })
}

func (c *Config) GetRegistryMap() map[string]*Registry {
	return toMapPtr(c.Registries, func(r Registry) string { return r.Name })
}

func (c *Config) GetDomainMap() map[string]*Domain {
	return toMapPtr(c.Domains, func(d Domain) string { return d.Name })
}

func (c *Config) GetAllDNSRecords() []DNSRecord {
	var records []DNSRecord
	for _, d := range c.Domains {
		records = append(records, d.FlattenRecords()...)
	}
	return records
}

func ParsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", domain.ErrInvalidPort, s)
	}
	if port <= 0 || port > constants.MaxPortNumber {
		return 0, fmt.Errorf("%w: %d", domain.ErrInvalidPort, port)
	}
	return port, nil
}
