package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/entities"
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

type Loader struct {
	env     string
	baseDir string
	config  *entities.Config
}

func NewLoader(env, baseDir string) *Loader {
	return &Loader{
		env:     env,
		baseDir: baseDir,
		config:  &entities.Config{},
	}
}

func (l *Loader) Load() (*entities.Config, error) {
	configDir := filepath.Join(l.baseDir, "userdata", l.env)

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("config directory does not exist: %s", configDir)
	}

	l.config = &entities.Config{}

	files := []struct {
		filename string
		loader   func(string) error
	}{
		{"secrets.yaml", l.loadSecrets},
		{"isps.yaml", l.loadISPs},
		{"zones.yaml", l.loadZones},
		{"gateways.yaml", l.loadGateways},
		{"servers.yaml", l.loadServers},
		{"services.yaml", l.loadServices},
		{"registries.yaml", l.loadRegistries},
		{"domains.yaml", l.loadDomains},
		{"dns.yaml", l.loadDNSRecords},
		{"certificates.yaml", l.loadCertificates},
	}

	for _, f := range files {
		filePath := filepath.Join(configDir, f.filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}
		if err := f.loader(filePath); err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", f.filename, err)
		}
	}

	return l.config, nil
}

func (l *Loader) loadSecrets(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type secretsFile struct {
		Secrets []entities.Secret `yaml:"secrets"`
	}

	var sf secretsFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Secrets = sf.Secrets
	return nil
}

func (l *Loader) loadISPs(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type ispsFile struct {
		ISPs []entities.ISP `yaml:"isps"`
	}

	var sf ispsFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.ISPs = sf.ISPs
	return nil
}

func (l *Loader) loadZones(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type zonesFile struct {
		Zones []entities.Zone `yaml:"zones"`
	}

	var sf zonesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Zones = sf.Zones
	return nil
}

func (l *Loader) loadGateways(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type gatewaysFile struct {
		Gateways []entities.Gateway `yaml:"gateways"`
	}

	var sf gatewaysFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Gateways = sf.Gateways
	return nil
}

func (l *Loader) loadServers(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type serversFile struct {
		Servers []entities.Server `yaml:"servers"`
	}

	var sf serversFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Servers = sf.Servers
	return nil
}

func (l *Loader) loadServices(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type servicesFile struct {
		Services []entities.Service `yaml:"services"`
	}

	var sf servicesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Services = sf.Services
	return nil
}

func (l *Loader) loadRegistries(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type registriesFile struct {
		Registries []entities.Registry `yaml:"registries"`
	}

	var sf registriesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Registries = sf.Registries
	return nil
}

func (l *Loader) loadDomains(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type domainsFile struct {
		Domains []entities.Domain `yaml:"domains"`
	}

	var sf domainsFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Domains = sf.Domains
	return nil
}

func (l *Loader) loadDNSRecords(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type dnsFile struct {
		Records []entities.DNSRecord `yaml:"records"`
	}

	var sf dnsFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.DNSRecords = sf.Records
	return nil
}

func (l *Loader) loadCertificates(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	type certificatesFile struct {
		Certificates []entities.Certificate `yaml:"certificates"`
	}

	var sf certificatesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return err
	}

	l.config.Certificates = sf.Certificates
	return nil
}

func (l *Loader) Validate() error {
	if l.config == nil {
		return ErrConfigNotLoaded
	}

	if err := l.config.Validate(); err != nil {
		return err
	}

	if err := l.validateReferences(); err != nil {
		return err
	}

	if err := l.validatePortConflicts(); err != nil {
		return err
	}

	if err := l.validateDomainConflicts(); err != nil {
		return err
	}

	if err := l.validateHostnameConflicts(); err != nil {
		return err
	}

	return nil
}

func (l *Loader) validateReferences() error {
	secrets := l.config.GetSecretsMap()
	isps := l.config.GetISPMap()
	zones := l.config.GetZoneMap()
	servers := l.config.GetServerMap()
	registries := l.config.GetRegistryMap()
	domains := l.config.GetDomainMap()

	if err := l.validateISPReferences(isps); err != nil {
		return err
	}

	if err := l.validateZoneReferences(zones, isps); err != nil {
		return err
	}

	if err := l.validateGatewayReferences(zones, servers); err != nil {
		return err
	}

	if err := l.validateServerReferences(zones, isps, registries); err != nil {
		return err
	}

	if err := l.validateServiceReferences(servers, secrets); err != nil {
		return err
	}

	if err := l.validateDomainReferences(isps, domains); err != nil {
		return err
	}

	if err := l.validateDNSReferences(domains); err != nil {
		return err
	}

	if err := l.validateCertificateReferences(domains); err != nil {
		return err
	}

	return nil
}

func (l *Loader) validateISPReferences(isps map[string]*entities.ISP) error {
	for _, isp := range l.config.ISPs {
		for _, ref := range isp.Credentials {
			if ref.Secret != "" {
				if _, ok := isps[ref.Secret]; ok {
					continue
				}
				secrets := l.config.GetSecretsMap()
				if _, ok := secrets[ref.Secret]; !ok {
					return fmt.Errorf("%w: secret '%s' referenced by isp '%s' does not exist", ErrMissingReference, ref.Secret, isp.Name)
				}
			}
		}
	}
	return nil
}

func (l *Loader) validateZoneReferences(zones map[string]*entities.Zone, isps map[string]*entities.ISP) error {
	for _, zone := range l.config.Zones {
		if _, ok := isps[zone.ISP]; !ok {
			return fmt.Errorf("%w: isp '%s' referenced by zone '%s' does not exist", ErrMissingReference, zone.ISP, zone.Name)
		}
	}
	return nil
}

func (l *Loader) validateGatewayReferences(zones map[string]*entities.Zone, servers map[string]*entities.Server) error {
	for _, gateway := range l.config.Gateways {
		if _, ok := zones[gateway.Zone]; !ok {
			return fmt.Errorf("%w: zone '%s' referenced by gateway '%s' does not exist", ErrMissingReference, gateway.Zone, gateway.Name)
		}
		if _, ok := servers[gateway.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by gateway '%s' does not exist", ErrMissingReference, gateway.Server, gateway.Name)
		}
	}
	return nil
}

func (l *Loader) validateServerReferences(zones map[string]*entities.Zone, isps map[string]*entities.ISP, registries map[string]*entities.Registry) error {
	for _, server := range l.config.Servers {
		if _, ok := zones[server.Zone]; !ok {
			return fmt.Errorf("%w: zone '%s' referenced by server '%s' does not exist", ErrMissingReference, server.Zone, server.Name)
		}
		if _, ok := isps[server.ISP]; !ok {
			return fmt.Errorf("%w: isp '%s' referenced by server '%s' does not exist", ErrMissingReference, server.ISP, server.Name)
		}
		for _, regName := range server.Environment.Registries {
			if _, ok := registries[regName]; !ok {
				return fmt.Errorf("%w: registry '%s' referenced by server '%s' does not exist", ErrMissingReference, regName, server.Name)
			}
		}
		secrets := l.config.GetSecretsMap()
		if server.SSH.Password.Secret != "" {
			if _, ok := secrets[server.SSH.Password.Secret]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by server '%s' ssh password does not exist", ErrMissingReference, server.SSH.Password.Secret, server.Name)
			}
		}
	}
	return nil
}

func (l *Loader) validateServiceReferences(servers map[string]*entities.Server, secrets map[string]string) error {
	for _, service := range l.config.Services {
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

func (l *Loader) validateDomainReferences(isps map[string]*entities.ISP, domains map[string]*entities.Domain) error {
	for _, domain := range l.config.Domains {
		if _, ok := isps[domain.ISP]; !ok {
			return fmt.Errorf("%w: isp '%s' referenced by domain '%s' does not exist", ErrMissingReference, domain.ISP, domain.Name)
		}
		if domain.Parent != "" {
			if _, ok := domains[domain.Parent]; !ok {
				return fmt.Errorf("%w: parent domain '%s' referenced by domain '%s' does not exist", ErrMissingReference, domain.Parent, domain.Name)
			}
		}
	}
	return nil
}

func (l *Loader) validateDNSReferences(domains map[string]*entities.Domain) error {
	for _, record := range l.config.DNSRecords {
		if _, ok := domains[record.Domain]; !ok {
			return fmt.Errorf("%w: domain '%s' referenced by dns record does not exist", ErrMissingReference, record.Domain)
		}
	}
	return nil
}

func (l *Loader) validateCertificateReferences(domains map[string]*entities.Domain) error {
	for _, cert := range l.config.Certificates {
		for _, domainName := range cert.Domains {
			if _, ok := domains[domainName]; !ok {
				return fmt.Errorf("%w: domain '%s' referenced by certificate '%s' does not exist", ErrMissingReference, domainName, cert.Name)
			}
		}
	}
	return nil
}

func (l *Loader) validatePortConflicts() error {
	serverPorts := make(map[string]map[int]string)

	for _, gateway := range l.config.Gateways {
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

	servicePorts := make(map[string]map[int]string)
	for _, service := range l.config.Services {
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

func (l *Loader) validateDomainConflicts() error {
	domainNames := make(map[string]string)
	for _, domain := range l.config.Domains {
		if existing, ok := domainNames[domain.Name]; ok {
			return fmt.Errorf("%w: domain '%s' is defined multiple times (first: '%s')", ErrDomainConflict, domain.Name, existing)
		}
		domainNames[domain.Name] = domain.Name
	}

	dnsKeys := make(map[string]string)
	for _, record := range l.config.DNSRecords {
		key := fmt.Sprintf("%s:%s:%s", record.Domain, record.Type, record.Name)
		if existing, ok := dnsKeys[key]; ok {
			return fmt.Errorf("%w: dns record '%s' is defined multiple times (type: %s, name: %s)", ErrDNSSubdomainConflict, record.Domain, existing, record.Name)
		}
		dnsKeys[key] = record.Name
	}

	return nil
}

func (l *Loader) validateHostnameConflicts() error {
	hostnames := make(map[string]string)
	for _, service := range l.config.Services {
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

func (l *Loader) GetConfig() *entities.Config {
	return l.config
}
