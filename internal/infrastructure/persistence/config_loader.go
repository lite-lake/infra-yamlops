package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"gopkg.in/yaml.v3"
)

var (
	ErrConfigNotLoaded      = errors.New("config not loaded")
	ErrMissingReference     = errors.New("missing reference")
	ErrPortConflict         = errors.New("port conflict")
	ErrDomainConflict       = errors.New("domain conflict")
	ErrHostnameConflict     = errors.New("hostname conflict")
	ErrDNSSubdomainConflict = errors.New("dns subdomain conflict")
)

type ConfigLoader struct {
	baseDir string
}

func NewConfigLoader(baseDir string) *ConfigLoader {
	return &ConfigLoader{baseDir: baseDir}
}

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
		{"domains.yaml", loadDomains},
		{"dns.yaml", loadDNSRecords},
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
	if cfg == nil {
		return ErrConfigNotLoaded
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := validateReferences(cfg); err != nil {
		return err
	}

	if err := validatePortConflicts(cfg); err != nil {
		return err
	}

	if err := validateDomainConflicts(cfg); err != nil {
		return err
	}

	if err := validateHostnameConflicts(cfg); err != nil {
		return err
	}

	return nil
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

func loadSecrets(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Secret](filePath, "secrets")
	if err != nil {
		return err
	}
	cfg.Secrets = items
	return nil
}

func loadISPs(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.ISP](filePath, "isps")
	if err != nil {
		return err
	}
	cfg.ISPs = items
	return nil
}

func loadZones(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Zone](filePath, "zones")
	if err != nil {
		return err
	}
	cfg.Zones = items
	return nil
}

func loadInfraServices(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.InfraService](filePath, "infra_services")
	if err != nil {
		return err
	}
	cfg.InfraServices = items
	return nil
}

func loadGateways(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Gateway](filePath, "gateways")
	if err != nil {
		return err
	}
	cfg.Gateways = items
	return nil
}

func loadServers(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Server](filePath, "servers")
	if err != nil {
		return err
	}
	cfg.Servers = items
	return nil
}

func loadServices(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.BizService](filePath, "services")
	if err != nil {
		return err
	}
	cfg.Services = items
	return nil
}

func loadRegistries(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Registry](filePath, "registries")
	if err != nil {
		return err
	}
	cfg.Registries = items
	return nil
}

func loadDomains(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Domain](filePath, "domains")
	if err != nil {
		return err
	}
	cfg.Domains = items
	return nil
}

func loadDNSRecords(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.DNSRecord](filePath, "records")
	if err != nil {
		return err
	}
	cfg.DNSRecords = items
	return nil
}

func loadCertificates(filePath string, cfg *entity.Config) error {
	items, err := loadEntity[entity.Certificate](filePath, "certificates")
	if err != nil {
		return err
	}
	cfg.Certificates = items
	return nil
}

func validateReferences(cfg *entity.Config) error {
	secrets := cfg.GetSecretsMap()
	isps := cfg.GetISPMap()
	zones := cfg.GetZoneMap()
	servers := cfg.GetServerMap()
	registries := cfg.GetRegistryMap()
	domains := cfg.GetDomainMap()

	if err := validateISPReferences(cfg, isps); err != nil {
		return err
	}

	if err := validateZoneReferences(cfg, isps); err != nil {
		return err
	}

	if err := validateInfraServiceReferences(cfg, servers); err != nil {
		return err
	}

	if err := validateGatewayReferences(cfg, zones, servers); err != nil {
		return err
	}

	if err := validateServerReferences(cfg, zones, isps, registries); err != nil {
		return err
	}

	if err := validateServiceReferences(cfg, servers, secrets); err != nil {
		return err
	}

	if err := validateDomainReferences(cfg, isps, domains); err != nil {
		return err
	}

	if err := validateDNSReferences(cfg, domains); err != nil {
		return err
	}

	if err := validateCertificateReferences(cfg, domains); err != nil {
		return err
	}

	return nil
}

func validateISPReferences(cfg *entity.Config, isps map[string]*entity.ISP) error {
	for _, isp := range cfg.ISPs {
		for _, ref := range isp.Credentials {
			if ref.Secret != "" {
				if _, ok := isps[ref.Secret]; ok {
					continue
				}
				secrets := cfg.GetSecretsMap()
				if _, ok := secrets[ref.Secret]; !ok {
					return fmt.Errorf("%w: secret '%s' referenced by isp '%s' does not exist", ErrMissingReference, ref.Secret, isp.Name)
				}
			}
		}
	}
	return nil
}

func validateZoneReferences(cfg *entity.Config, isps map[string]*entity.ISP) error {
	for _, zone := range cfg.Zones {
		if zone.ISP != "" {
			if _, ok := isps[zone.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by zone '%s' does not exist", ErrMissingReference, zone.ISP, zone.Name)
			}
		}
	}
	return nil
}

func validateInfraServiceReferences(cfg *entity.Config, servers map[string]*entity.Server) error {
	for _, infra := range cfg.InfraServices {
		if _, ok := servers[infra.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by infra_service '%s' does not exist", ErrMissingReference, infra.Server, infra.Name)
		}
	}
	return nil
}

func validateGatewayReferences(cfg *entity.Config, zones map[string]*entity.Zone, servers map[string]*entity.Server) error {
	for _, gateway := range cfg.Gateways {
		if _, ok := zones[gateway.Zone]; !ok {
			return fmt.Errorf("%w: zone '%s' referenced by gateway '%s' does not exist", ErrMissingReference, gateway.Zone, gateway.Name)
		}
		if _, ok := servers[gateway.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by gateway '%s' does not exist", ErrMissingReference, gateway.Server, gateway.Name)
		}
	}
	return nil
}

func validateServerReferences(cfg *entity.Config, zones map[string]*entity.Zone, isps map[string]*entity.ISP, registries map[string]*entity.Registry) error {
	for _, server := range cfg.Servers {
		if _, ok := zones[server.Zone]; !ok {
			return fmt.Errorf("%w: zone '%s' referenced by server '%s' does not exist", ErrMissingReference, server.Zone, server.Name)
		}
		if server.ISP != "" {
			if _, ok := isps[server.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by server '%s' does not exist", ErrMissingReference, server.ISP, server.Name)
			}
		}
		for _, regName := range server.Environment.Registries {
			if _, ok := registries[regName]; !ok {
				return fmt.Errorf("%w: registry '%s' referenced by server '%s' does not exist", ErrMissingReference, regName, server.Name)
			}
		}
		secrets := cfg.GetSecretsMap()
		if server.SSH.Password.Secret != "" {
			if _, ok := secrets[server.SSH.Password.Secret]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by server '%s' ssh password does not exist", ErrMissingReference, server.SSH.Password.Secret, server.Name)
			}
		}
	}
	return nil
}

func validateServiceReferences(cfg *entity.Config, servers map[string]*entity.Server, secrets map[string]string) error {
	for _, service := range cfg.Services {
		if _, ok := servers[service.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by service '%s' does not exist", ErrMissingReference, service.Server, service.Name)
		}
		for _, secretName := range service.Secrets {
			if _, ok := secrets[secretName]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by service '%s' does not exist", ErrMissingReference, secretName, service.Name)
			}
		}
	}
	return nil
}

func validateDomainReferences(cfg *entity.Config, isps map[string]*entity.ISP, domains map[string]*entity.Domain) error {
	for _, domain := range cfg.Domains {
		if domain.ISP != "" {
			if _, ok := isps[domain.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by domain '%s' does not exist", ErrMissingReference, domain.ISP, domain.Name)
			}
		}
		if domain.DNSISP != "" {
			if _, ok := isps[domain.DNSISP]; !ok {
				return fmt.Errorf("%w: dns_isp '%s' referenced by domain '%s' does not exist", ErrMissingReference, domain.DNSISP, domain.Name)
			}
		}
		if domain.Parent != "" {
			if _, ok := domains[domain.Parent]; !ok {
				return fmt.Errorf("%w: parent domain '%s' referenced by domain '%s' does not exist", ErrMissingReference, domain.Parent, domain.Name)
			}
		}
	}
	return nil
}

func validateDNSReferences(cfg *entity.Config, domains map[string]*entity.Domain) error {
	for _, record := range cfg.DNSRecords {
		if _, ok := domains[record.Domain]; !ok {
			return fmt.Errorf("%w: domain '%s' referenced by dns record does not exist", ErrMissingReference, record.Domain)
		}
	}
	return nil
}

func validateCertificateReferences(cfg *entity.Config, domains map[string]*entity.Domain) error {
	for _, cert := range cfg.Certificates {
		for _, domainName := range cert.Domains {
			if _, ok := domains[domainName]; !ok {
				return fmt.Errorf("%w: domain '%s' referenced by certificate '%s' does not exist", ErrMissingReference, domainName, cert.Name)
			}
		}
	}
	return nil
}

func validatePortConflicts(cfg *entity.Config) error {
	serverPorts := make(map[string]map[int]string)

	for _, gateway := range cfg.Gateways {
		key := gateway.Server
		if serverPorts[key] == nil {
			serverPorts[key] = make(map[int]string)
		}
		if existing, ok := serverPorts[key][gateway.Ports.HTTP]; ok {
			return fmt.Errorf("%w: http port %d on server '%s' is used by both '%s' and '%s'", ErrPortConflict, gateway.Ports.HTTP, gateway.Server, existing, gateway.Name)
		}
		serverPorts[key][gateway.Ports.HTTP] = gateway.Name
		if existing, ok := serverPorts[key][gateway.Ports.HTTPS]; ok {
			return fmt.Errorf("%w: https port %d on server '%s' is used by both '%s' and '%s'", ErrPortConflict, gateway.Ports.HTTPS, gateway.Server, existing, gateway.Name)
		}
		serverPorts[key][gateway.Ports.HTTPS] = gateway.Name
	}

	for _, infra := range cfg.InfraServices {
		key := infra.Server
		if serverPorts[key] == nil {
			serverPorts[key] = make(map[int]string)
		}
		if infra.SSLConfig != nil && infra.SSLConfig.Ports.API > 0 {
			if existing, ok := serverPorts[key][infra.SSLConfig.Ports.API]; ok {
				return fmt.Errorf("%w: api port %d on server '%s' is used by both '%s' and '%s'", ErrPortConflict, infra.SSLConfig.Ports.API, infra.Server, existing, infra.Name)
			}
			serverPorts[key][infra.SSLConfig.Ports.API] = infra.Name
		}
	}

	servicePorts := make(map[string]map[int]string)
	for _, service := range cfg.Services {
		key := service.Server
		if servicePorts[key] == nil {
			servicePorts[key] = make(map[int]string)
		}
		for _, port := range service.Ports {
			if existing, ok := servicePorts[key][port.Host]; ok {
				return fmt.Errorf("%w: host port %d on server '%s' is used by both services '%s' and '%s'", ErrPortConflict, port.Host, service.Server, existing, service.Name)
			}
			servicePorts[key][port.Host] = service.Name
		}
	}

	return nil
}

func validateDomainConflicts(cfg *entity.Config) error {
	domainNames := make(map[string]string)
	for _, domain := range cfg.Domains {
		if existing, ok := domainNames[domain.Name]; ok {
			return fmt.Errorf("%w: domain '%s' is defined multiple times (first: '%s')", ErrDomainConflict, domain.Name, existing)
		}
		domainNames[domain.Name] = domain.Name
	}

	dnsKeys := make(map[string]string)
	for _, record := range cfg.DNSRecords {
		key := fmt.Sprintf("%s:%s:%s", record.Domain, record.Type, record.Name)
		if existing, ok := dnsKeys[key]; ok {
			return fmt.Errorf("%w: dns record '%s' is defined multiple times (type: %s, name: %s)", ErrDNSSubdomainConflict, record.Domain, existing, record.Name)
		}
		dnsKeys[key] = record.Name
	}

	return nil
}

func validateHostnameConflicts(cfg *entity.Config) error {
	hostnames := make(map[string]string)
	for _, service := range cfg.Services {
		for _, route := range service.Gateways {
			if route.HasGateway() && route.Hostname != "" {
				hostname := strings.ToLower(route.Hostname)
				if existing, ok := hostnames[hostname]; ok {
					return fmt.Errorf("%w: hostname '%s' is used by both services '%s' and '%s'", ErrHostnameConflict, hostname, existing, service.Name)
				}
				hostnames[hostname] = service.Name
			}
		}
	}
	return nil
}

var _ repository.ConfigLoader = (*ConfigLoader)(nil)
