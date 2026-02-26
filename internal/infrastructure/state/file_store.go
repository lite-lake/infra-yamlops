package state

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"gopkg.in/yaml.v3"
)

type FileStore struct {
	path  string
	flock *flock.Flock
}

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:  path,
		flock: flock.New(path + ".lock"),
	}
}

func (s *FileStore) Load(ctx context.Context, env string) (*repository.DeploymentState, error) {
	if err := s.flock.Lock(); err != nil {
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}
	defer s.flock.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return repository.NewDeploymentState(), nil
		}
		return nil, fmt.Errorf("reading state file %s: %w", s.path, domain.WrapOp("read state file", domain.ErrStateReadFailed))
	}

	var cfg entity.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing state file %s: %w", s.path, domain.WrapOp("parse state file", domain.ErrStateSerializeFail))
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
	for i := range cfg.ISPs {
		state.ISPs[cfg.ISPs[i].Name] = &cfg.ISPs[i]
	}

	return state, nil
}

func (s *FileStore) Save(ctx context.Context, env string, state *repository.DeploymentState) error {
	if err := s.flock.Lock(); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer s.flock.Unlock()

	cfg := &entity.Config{
		Services:      make([]entity.BizService, 0, len(state.Services)),
		InfraServices: make([]entity.InfraService, 0, len(state.InfraServices)),
		Servers:       make([]entity.Server, 0, len(state.Servers)),
		Zones:         make([]entity.Zone, 0, len(state.Zones)),
		Domains:       make([]entity.Domain, 0, len(state.Domains)),
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
	for _, isp := range state.ISPs {
		cfg.ISPs = append(cfg.ISPs, *isp)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling state for %s: %w", s.path, domain.WrapOp("marshal state", domain.ErrStateSerializeFail))
	}

	tmpPath := filepath.Join(filepath.Dir(s.path), "."+filepath.Base(s.path)+".tmp")
	if err := os.WriteFile(tmpPath, data, constants.FilePermissionOwnerRW); err != nil {
		return fmt.Errorf("writing temp state file %s: %w", tmpPath, domain.WrapOp("write temp state file", domain.ErrStateWriteFailed))
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming state file from %s to %s: %w", tmpPath, s.path, domain.WrapOp("rename state file", domain.ErrStateWriteFailed))
	}

	return nil
}
