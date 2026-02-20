package state

import (
	"fmt"
	"os"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"gopkg.in/yaml.v3"
)

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Load() (*repository.DeploymentState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var cfg entity.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	state := repository.NewDeploymentState()

	for i := range cfg.Services {
		state.Services[cfg.Services[i].Name] = &cfg.Services[i]
	}
	for i := range cfg.InfraServices {
		state.InfraServices[cfg.InfraServices[i].Name] = &cfg.InfraServices[i]
	}
	for i := range cfg.Servers {
		state.Servers[cfg.Servers[i].Name] = &cfg.Servers[i]
	}
	for i := range cfg.Zones {
		state.Zones[cfg.Zones[i].Name] = &cfg.Zones[i]
	}
	for i := range cfg.Domains {
		state.Domains[cfg.Domains[i].Name] = &cfg.Domains[i]
		for _, r := range cfg.Domains[i].FlattenRecords() {
			record := r
			key := fmt.Sprintf("%s:%s:%s", record.Domain, record.Type, record.Name)
			state.Records[key] = &record
		}
	}
	for i := range cfg.Certificates {
		state.Certs[cfg.Certificates[i].Name] = &cfg.Certificates[i]
	}
	for i := range cfg.Registries {
		state.Registries[cfg.Registries[i].Name] = &cfg.Registries[i]
	}
	for i := range cfg.ISPs {
		state.ISPs[cfg.ISPs[i].Name] = &cfg.ISPs[i]
	}

	return state, nil
}

func (s *FileStore) Save(state *repository.DeploymentState) error {
	cfg := &entity.Config{
		Services:      make([]entity.BizService, 0, len(state.Services)),
		InfraServices: make([]entity.InfraService, 0, len(state.InfraServices)),
		Servers:       make([]entity.Server, 0, len(state.Servers)),
		Zones:         make([]entity.Zone, 0, len(state.Zones)),
		Domains:       make([]entity.Domain, 0, len(state.Domains)),
		Certificates:  make([]entity.Certificate, 0, len(state.Certs)),
		Registries:    make([]entity.Registry, 0, len(state.Registries)),
		ISPs:          make([]entity.ISP, 0, len(state.ISPs)),
	}

	for _, svc := range state.Services {
		cfg.Services = append(cfg.Services, *svc)
	}
	for _, infra := range state.InfraServices {
		cfg.InfraServices = append(cfg.InfraServices, *infra)
	}
	for _, srv := range state.Servers {
		cfg.Servers = append(cfg.Servers, *srv)
	}
	for _, z := range state.Zones {
		cfg.Zones = append(cfg.Zones, *z)
	}
	for _, d := range state.Domains {
		cfg.Domains = append(cfg.Domains, *d)
	}
	for _, c := range state.Certs {
		cfg.Certificates = append(cfg.Certificates, *c)
	}
	for _, r := range state.Registries {
		cfg.Registries = append(cfg.Registries, *r)
	}
	for _, isp := range state.ISPs {
		cfg.ISPs = append(cfg.ISPs, *isp)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}
