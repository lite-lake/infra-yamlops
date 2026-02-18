package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
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
