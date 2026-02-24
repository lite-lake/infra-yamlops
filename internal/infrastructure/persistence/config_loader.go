package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/service"
	"github.com/litelake/yamlops/internal/infrastructure/logger"
	"gopkg.in/yaml.v3"
)

type ConfigLoader struct{ baseDir string }

func NewConfigLoader(baseDir string) *ConfigLoader { return &ConfigLoader{baseDir: baseDir} }

func (l *ConfigLoader) Load(ctx context.Context, env string) (*entity.Config, error) {
	log := logger.FromContext(ctx)

	configDir := filepath.Join(l.baseDir, "userdata", env)
	log.Debug("loading config", "env", env, "dir", configDir)

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		log.Error("config directory not found", "dir", configDir)
		return nil, fmt.Errorf("%w: %s", domainerr.ErrConfigNotFound, configDir)
	}

	cfg := &entity.Config{}
	loaders := []struct {
		filename string
		loader   func(string, *entity.Config) error
	}{
		{"secrets.yaml", loadSecrets},
		{"isps.yaml", loadISPs},
		{"zones.yaml", loadZones},
		{"services_infra.yaml", loadInfraServices},
		{"servers.yaml", loadServers},
		{"services_biz.yaml", loadServices},
		{"registries.yaml", loadRegistries},
		{"dns.yaml", loadDomains},
	}

	for _, f := range loaders {
		filePath := filepath.Join(configDir, f.filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Debug("config file skipped", "file", f.filename, "reason", "not found")
			continue
		}
		log.Debug("loading config file", "file", f.filename)
		if err := f.loader(filePath, cfg); err != nil {
			log.Error("failed to load config file", "file", f.filename, "error", err)
			return nil, fmt.Errorf("%w: %s: %w", domainerr.ErrConfigReadFailed, f.filename, err)
		}
	}

	log.Info("config loaded", "env", env)
	return cfg, nil
}

func (l *ConfigLoader) Validate(cfg *entity.Config) error {
	return service.NewValidator(cfg).Validate()
}

func loadEntity[T any](filePath, yamlKey string) ([]T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", filePath, err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML in %s: %w", filePath, err)
	}
	itemsRaw, ok := raw[yamlKey]
	if !ok {
		return nil, nil
	}
	itemsData, err := yaml.Marshal(itemsRaw)
	if err != nil {
		return nil, fmt.Errorf("marshaling %s items in %s: %w", yamlKey, filePath, err)
	}
	var items []T
	if err := yaml.Unmarshal(itemsData, &items); err != nil {
		return nil, fmt.Errorf("parsing %s items in %s: %w", yamlKey, filePath, err)
	}
	return items, nil
}

func loadSecrets(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Secret](fp, "secrets")
	if err != nil {
		return fmt.Errorf("loading secrets from %s: %w", fp, err)
	}
	cfg.Secrets = items
	return nil
}

func loadISPs(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.ISP](fp, "isps")
	if err != nil {
		return fmt.Errorf("loading ISPs from %s: %w", fp, err)
	}
	cfg.ISPs = items
	return nil
}

func loadZones(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Zone](fp, "zones")
	if err != nil {
		return fmt.Errorf("loading zones from %s: %w", fp, err)
	}
	cfg.Zones = items
	return nil
}

func loadInfraServices(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.InfraService](fp, "infra_services")
	if err != nil {
		return fmt.Errorf("loading infra services from %s: %w", fp, err)
	}
	cfg.InfraServices = items
	return nil
}

func loadServers(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Server](fp, "servers")
	if err != nil {
		return fmt.Errorf("loading servers from %s: %w", fp, err)
	}
	cfg.Servers = items
	return nil
}

func loadServices(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.BizService](fp, "services")
	if err != nil {
		return fmt.Errorf("loading services from %s: %w", fp, err)
	}
	cfg.Services = items
	return nil
}

func loadRegistries(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Registry](fp, "registries")
	if err != nil {
		return fmt.Errorf("loading registries from %s: %w", fp, err)
	}
	cfg.Registries = items
	return nil
}

func loadDomains(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Domain](fp, "domains")
	if err != nil {
		return fmt.Errorf("loading domains from %s: %w", fp, err)
	}
	cfg.Domains = items
	return nil
}

var _ repository.ConfigLoader = (*ConfigLoader)(nil)
