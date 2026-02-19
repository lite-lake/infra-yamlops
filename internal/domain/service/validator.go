package service

import (
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
)

type Validator struct {
	cfg *entity.Config

	secrets    map[string]string
	isps       map[string]*entity.ISP
	zones      map[string]*entity.Zone
	servers    map[string]*entity.Server
	registries map[string]*entity.Registry
	domainMap  map[string]*entity.Domain
}

func NewValidator(cfg *entity.Config) *Validator {
	if cfg == nil {
		return &Validator{cfg: nil}
	}
	return &Validator{
		cfg:        cfg,
		secrets:    cfg.GetSecretsMap(),
		isps:       cfg.GetISPMap(),
		zones:      cfg.GetZoneMap(),
		servers:    cfg.GetServerMap(),
		registries: cfg.GetRegistryMap(),
		domainMap:  cfg.GetDomainMap(),
	}
}

func (v *Validator) Validate() error {
	if v.cfg == nil {
		return domain.ErrConfigNotLoaded
	}

	if err := v.cfg.Validate(); err != nil {
		return err
	}

	if err := v.validateISPReferences(); err != nil {
		return err
	}

	if err := v.validateZoneReferences(); err != nil {
		return err
	}

	if err := v.validateInfraServiceReferences(); err != nil {
		return err
	}

	if err := v.validateServerReferences(); err != nil {
		return err
	}

	if err := v.validateServiceReferences(); err != nil {
		return err
	}

	if err := v.validateDomainReferences(); err != nil {
		return err
	}

	if err := v.validateDNSReferences(); err != nil {
		return err
	}

	if err := v.validateCertificateReferences(); err != nil {
		return err
	}

	if err := v.validatePortConflicts(); err != nil {
		return err
	}

	if err := v.validateDomainConflicts(); err != nil {
		return err
	}

	if err := v.validateHostnameConflicts(); err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateISPReferences() error {
	for _, isp := range v.cfg.ISPs {
		for _, ref := range isp.Credentials {
			if ref.Secret != "" {
				if _, ok := v.isps[ref.Secret]; ok {
					continue
				}
				if _, ok := v.secrets[ref.Secret]; !ok {
					return fmt.Errorf("%w: secret '%s' referenced by isp '%s' does not exist", domain.ErrMissingReference, ref.Secret, isp.Name)
				}
			}
		}
	}
	return nil
}

func (v *Validator) validateZoneReferences() error {
	for _, zone := range v.cfg.Zones {
		if zone.ISP != "" {
			if _, ok := v.isps[zone.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by zone '%s' does not exist", domain.ErrMissingReference, zone.ISP, zone.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateInfraServiceReferences() error {
	for _, infra := range v.cfg.InfraServices {
		if _, ok := v.servers[infra.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by infra_service '%s' does not exist", domain.ErrMissingReference, infra.Server, infra.Name)
		}
	}
	return nil
}

func (v *Validator) validateServerReferences() error {
	for _, server := range v.cfg.Servers {
		if _, ok := v.zones[server.Zone]; !ok {
			return fmt.Errorf("%w: zone '%s' referenced by server '%s' does not exist", domain.ErrMissingReference, server.Zone, server.Name)
		}
		if server.ISP != "" {
			if _, ok := v.isps[server.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by server '%s' does not exist", domain.ErrMissingReference, server.ISP, server.Name)
			}
		}
		for _, regName := range server.Environment.Registries {
			if _, ok := v.registries[regName]; !ok {
				return fmt.Errorf("%w: registry '%s' referenced by server '%s' does not exist", domain.ErrMissingReference, regName, server.Name)
			}
		}
		if server.SSH.Password.Secret != "" {
			if _, ok := v.secrets[server.SSH.Password.Secret]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by server '%s' ssh password does not exist", domain.ErrMissingReference, server.SSH.Password.Secret, server.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateServiceReferences() error {
	for _, service := range v.cfg.Services {
		if _, ok := v.servers[service.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by service '%s' does not exist", domain.ErrMissingReference, service.Server, service.Name)
		}
		for _, secretName := range service.Secrets {
			if _, ok := v.secrets[secretName]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by service '%s' does not exist", domain.ErrMissingReference, secretName, service.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateDomainReferences() error {
	for _, d := range v.cfg.Domains {
		if d.ISP != "" {
			if _, ok := v.isps[d.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by domain '%s' does not exist", domain.ErrMissingReference, d.ISP, d.Name)
			}
		}
		if d.DNSISP != "" {
			if _, ok := v.isps[d.DNSISP]; !ok {
				return fmt.Errorf("%w: dns_isp '%s' referenced by domain '%s' does not exist", domain.ErrMissingReference, d.DNSISP, d.Name)
			}
		}
		if d.Parent != "" {
			if _, ok := v.domainMap[d.Parent]; !ok {
				return fmt.Errorf("%w: parent domain '%s' referenced by domain '%s' does not exist", domain.ErrMissingReference, d.Parent, d.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateDNSReferences() error {
	for _, record := range v.cfg.GetAllDNSRecords() {
		if _, ok := v.domainMap[record.Domain]; !ok {
			return fmt.Errorf("%w: domain '%s' referenced by dns record does not exist", domain.ErrMissingReference, record.Domain)
		}
	}
	return nil
}

func (v *Validator) validateCertificateReferences() error {
	for _, cert := range v.cfg.Certificates {
		for _, domainName := range cert.Domains {
			if _, ok := v.domainMap[domainName]; !ok {
				return fmt.Errorf("%w: domain '%s' referenced by certificate '%s' does not exist", domain.ErrMissingReference, domainName, cert.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validatePortConflicts() error {
	serverPorts := make(map[string]map[int]string)

	for _, infra := range v.cfg.InfraServices {
		key := infra.Server
		if serverPorts[key] == nil {
			serverPorts[key] = make(map[int]string)
		}
		if infra.SSLConfig != nil && infra.SSLConfig.Ports.API > 0 {
			if existing, ok := serverPorts[key][infra.SSLConfig.Ports.API]; ok {
				return fmt.Errorf("%w: api port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, infra.SSLConfig.Ports.API, infra.Server, existing, infra.Name)
			}
			serverPorts[key][infra.SSLConfig.Ports.API] = infra.Name
		}
	}

	servicePorts := make(map[string]map[int]string)
	for _, service := range v.cfg.Services {
		key := service.Server
		if servicePorts[key] == nil {
			servicePorts[key] = make(map[int]string)
		}
		for _, port := range service.Ports {
			if existing, ok := servicePorts[key][port.Host]; ok {
				return fmt.Errorf("%w: host port %d on server '%s' is used by both services '%s' and '%s'", domain.ErrPortConflict, port.Host, service.Server, existing, service.Name)
			}
			servicePorts[key][port.Host] = service.Name
		}
	}

	return nil
}

func (v *Validator) validateDomainConflicts() error {
	domainNames := make(map[string]string)
	for _, d := range v.cfg.Domains {
		if existing, ok := domainNames[d.Name]; ok {
			return fmt.Errorf("%w: domain '%s' is defined multiple times (first: '%s')", domain.ErrDomainConflict, d.Name, existing)
		}
		domainNames[d.Name] = d.Name
	}

	dnsKeys := make(map[string]bool)
	for _, record := range v.cfg.GetAllDNSRecords() {
		key := fmt.Sprintf("%s:%s:%s:%s", record.Domain, record.Type, record.Name, record.Value)
		if dnsKeys[key] {
			return fmt.Errorf("%w: dns record '%s' is defined multiple times (type: %s, name: %s, value: %s)", domain.ErrDNSSubdomainConflict, record.Domain, record.Type, record.Name, record.Value)
		}
		dnsKeys[key] = true
	}

	return nil
}

func (v *Validator) validateHostnameConflicts() error {
	hostnames := make(map[string]string)
	for _, service := range v.cfg.Services {
		for _, route := range service.Gateways {
			if route.HasGateway() && route.Hostname != "" {
				hostname := strings.ToLower(route.Hostname)
				if existing, ok := hostnames[hostname]; ok {
					return fmt.Errorf("%w: hostname '%s' is used by both services '%s' and '%s'", domain.ErrHostnameConflict, hostname, existing, service.Name)
				}
				hostnames[hostname] = service.Name
			}
		}
	}
	return nil
}
