package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (s *PlannerService) PlanRecords(plan *valueobject.Plan, cfgRecords []entity.DNSRecord, scope *valueobject.Scope) {
	recordKey := func(r *entity.DNSRecord) string {
		return fmt.Sprintf("%s:%s:%s", r.Domain, r.Type, r.Name)
	}

	stateMap := make(map[string]*entity.DNSRecord)
	for key, r := range s.state.Records {
		stateMap[key] = r
	}

	cfgMap := make(map[string]*entity.DNSRecord)
	for i := range cfgRecords {
		key := recordKey(&cfgRecords[i])
		cfgMap[key] = &cfgRecords[i]
	}

	for key, state := range stateMap {
		if _, exists := cfgMap[key]; !exists {
			if scope.Matches("", "", "", state.Domain) {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "dns_record",
					Name:     key,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete dns record %s", key)},
				})
			}
		}
	}

	for key, cfg := range cfgMap {
		if state, exists := stateMap[key]; exists {
			if !RecordEquals(state, cfg) {
				if scope.Matches("", "", "", cfg.Domain) {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "dns_record",
						Name:     key,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update dns record %s", key)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", cfg.Domain) {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "dns_record",
					Name:     key,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create dns record %s", key)},
				})
			}
		}
	}
}

func RecordEquals(a, b *entity.DNSRecord) bool {
	return a.Domain == b.Domain && a.Type == b.Type && a.Name == b.Name && a.Value == b.Value && a.TTL == b.TTL
}

func (s *PlannerService) PlanCertificates(plan *valueobject.Plan, cfgMap map[string]*entity.Certificate, scope *valueobject.Scope) {
	for name, state := range s.state.Certs {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "certificate",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete certificate %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := s.state.Certs[name]; exists {
			if !CertificateEquals(state, cfg) {
				if scope.Matches("", "", "", "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "certificate",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update certificate %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "certificate",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create certificate %s", name)},
				})
			}
		}
	}
}

func CertificateEquals(a, b *entity.Certificate) bool {
	if a.Name != b.Name || a.Provider != b.Provider || a.DNSProvider != b.DNSProvider {
		return false
	}
	if len(a.Domains) != len(b.Domains) {
		return false
	}
	for i, d := range a.Domains {
		if i >= len(b.Domains) || d != b.Domains[i] {
			return false
		}
	}
	return true
}

func (s *PlannerService) PlanRegistries(plan *valueobject.Plan, cfgMap map[string]*entity.Registry, scope *valueobject.Scope) {
	for name, state := range s.state.Registries {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   "registry",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete registry %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := s.state.Registries[name]; exists {
			if !RegistryEquals(state, cfg) {
				if scope.Matches("", "", "", "") {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   "registry",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update registry %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   "registry",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create registry %s", name)},
				})
			}
		}
	}
}

func RegistryEquals(a, b *entity.Registry) bool {
	return a.Name == b.Name && a.URL == b.URL
}

func NewDeploymentState() *repository.DeploymentState {
	return &repository.DeploymentState{
		Services:   make(map[string]*entity.BizService),
		Gateways:   make(map[string]*entity.Gateway),
		Servers:    make(map[string]*entity.Server),
		Zones:      make(map[string]*entity.Zone),
		Domains:    make(map[string]*entity.Domain),
		Records:    make(map[string]*entity.DNSRecord),
		Certs:      make(map[string]*entity.Certificate),
		Registries: make(map[string]*entity.Registry),
		ISPs:       make(map[string]*entity.ISP),
	}
}
