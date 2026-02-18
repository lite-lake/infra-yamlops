package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/service"
	"gopkg.in/yaml.v3"
)

type ConfigLoader struct{ baseDir string }

func NewConfigLoader(baseDir string) *ConfigLoader { return &ConfigLoader{baseDir: baseDir} }

func (l *ConfigLoader) Load(ctx context.Context, env string) (*entity.Config, error) {
	configDir := filepath.Join(l.baseDir, "userdata", env)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("config directory does not exist: %s", configDir)
	}

	cfg := &entity.Config{}
	loaders := []struct {
		filename string
		loader   func(string, *entity.Config) error
	}{
		{"secrets.yaml", loadSecrets},
		{"isps.yaml", loadISPs},
		{"zones.yaml", loadZones},
		{"infra_services.yaml", loadInfraServices},
		{"gateways.yaml", loadGateways},
		{"servers.yaml", loadServers},
		{"services.yaml", loadServices},
		{"registries.yaml", loadRegistries},
		{"dns.yaml", loadDomains},
		{"certificates.yaml", loadCertificates},
	}

	for _, f := range loaders {
		filePath := filepath.Join(configDir, f.filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}
		if err := f.loader(filePath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", f.filename, err)
		}
	}
	return cfg, nil
}

func (l *ConfigLoader) Validate(cfg *entity.Config) error {
	return service.NewValidator(cfg).Validate()
}

func loadEntity[T any](filePath, yamlKey string) ([]T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	itemsRaw, ok := raw[yamlKey]
	if !ok {
		return nil, nil
	}
	itemsData, err := yaml.Marshal(itemsRaw)
	if err != nil {
		return nil, err
	}
	var items []T
	if err := yaml.Unmarshal(itemsData, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func loadSecrets(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Secret](fp, "secrets")
	if err != nil {
		return err
	}
	cfg.Secrets = items
	return nil
}

func loadISPs(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.ISP](fp, "isps")
	if err != nil {
		return err
	}
	cfg.ISPs = items
	return nil
}

func loadZones(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Zone](fp, "zones")
	if err != nil {
		return err
	}
	cfg.Zones = items
	return nil
}

func loadInfraServices(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.InfraService](fp, "infra_services")
	if err != nil {
		return err
	}
	cfg.InfraServices = items
	return nil
}

func loadGateways(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Gateway](fp, "gateways")
	if err != nil {
		return err
	}
	cfg.Gateways = items
	return nil
}

func loadServers(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Server](fp, "servers")
	if err != nil {
		return err
	}
	cfg.Servers = items
	return nil
}

func loadServices(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.BizService](fp, "services")
	if err != nil {
		return err
	}
	cfg.Services = items
	return nil
}

func loadRegistries(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Registry](fp, "registries")
	if err != nil {
		return err
	}
	cfg.Registries = items
	return nil
}

func loadDomains(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Domain](fp, "domains")
	if err != nil {
		return err
	}
	cfg.Domains = items
	return nil
}

func loadCertificates(fp string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Certificate](fp, "certificates")
	if err != nil {
		return err
	}
	cfg.Certificates = items
	return nil
}

var _ repository.ConfigLoader = (*ConfigLoader)(nil)
