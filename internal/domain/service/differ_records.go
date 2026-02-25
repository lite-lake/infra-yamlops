package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (s *DifferService) PlanRecords(plan *valueobject.Plan, cfgRecords []entity.DNSRecord, scope *valueobject.Scope) {
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
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeDelete,
					"dns_record",
					key,
					state,
					nil,
					[]string{fmt.Sprintf("delete dns record %s", key)},
					false,
				))
			}
		}
	}

	for key, cfg := range cfgMap {
		if state, exists := stateMap[key]; exists {
			if !RecordEquals(state, cfg) {
				if scope.Matches("", "", "", cfg.Domain) {
					plan.AddChange(valueobject.NewChangeFull(
						valueobject.ChangeTypeUpdate,
						"dns_record",
						key,
						state,
						cfg,
						[]string{fmt.Sprintf("update dns record %s", key)},
						false,
					))
				}
			}
		} else {
			if scope.Matches("", "", "", cfg.Domain) {
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeCreate,
					"dns_record",
					key,
					nil,
					cfg,
					[]string{fmt.Sprintf("create dns record %s", key)},
					false,
				))
			}
		}
	}
}

func RecordEquals(a, b *entity.DNSRecord) bool {
	return a.Domain == b.Domain && a.Type == b.Type && a.Name == b.Name && a.Value == b.Value && a.TTL == b.TTL
}
