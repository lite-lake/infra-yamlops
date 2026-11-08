package service

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func (s *DifferService) PlanRecords(plan *valueobject.Plan, cfgRecords []entity.DNSRecord, scope *valueobject.Scope) {
	// 记录身份键含 Value：同名同线路多值记录（如 @ TXT 同时挂 SPF 与站点验证）是合法 DNS 形态，
	// 键不含 Value 时后写覆盖前写、静默丢记录（v1.0.26 及之前的行为，dry-run 检不出）。
	recordKey := func(r *entity.DNSRecord) string {
		return fmt.Sprintf("%s:%s:%s:%s:%s", r.Domain, r.Type, r.Name, r.EffectiveLine(), r.Value)
	}
	// 同名同线路记录组（不含 Value），用于多值组的增删与 1:1 原地 update 判定
	groupKey := func(r *entity.DNSRecord) string {
		return fmt.Sprintf("%s:%s:%s:%s", r.Domain, r.Type, r.Name, r.EffectiveLine())
	}

	stateMap := make(map[string]*entity.DNSRecord)
	for key, r := range s.state.Records {
		stateMap[key] = r
	}

	cfgMap := make(map[string]*entity.DNSRecord)
	for i := range cfgRecords {
		cfgMap[recordKey(&cfgRecords[i])] = &cfgRecords[i]
	}

	// 精确匹配（含 Value）之外的剩余记录按组归集，统一判定 update 或 create/delete
	restState := make(map[string][]*entity.DNSRecord)
	restCfg := make(map[string][]*entity.DNSRecord)

	for key, state := range stateMap {
		cfg, exists := cfgMap[key]
		if !exists {
			g := groupKey(state)
			restState[g] = append(restState[g], state)
			continue
		}
		if !RecordEquals(state, cfg) {
			// 键相同仅剩 TTL 差异 → 原地 update
			recordID := fmt.Sprintf("%s:%s", cfg.Type, cfg.Name)
			if scope.Matches("", "", "", "", cfg.Domain, recordID) {
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
	}
	for i := range cfgRecords {
		cfg := &cfgRecords[i]
		if _, exists := stateMap[recordKey(cfg)]; !exists {
			g := groupKey(cfg)
			restCfg[g] = append(restCfg[g], cfg)
		}
	}

	// 组内恰好 1 旧值 ↔ 1 新值 → 原地 update（保留 IP 变更 in-place 语义，避免 delete+create 解析空窗）；
	// 其余多值形态 → 旧值 delete、新值 create 收敛
	consumed := make(map[string]bool)
	for g, stateList := range restState {
		cfgList := restCfg[g]
		if len(stateList) == 1 && len(cfgList) == 1 {
			state, cfg := stateList[0], cfgList[0]
			key := recordKey(cfg)
			recordID := fmt.Sprintf("%s:%s", cfg.Type, cfg.Name)
			if scope.Matches("", "", "", "", cfg.Domain, recordID) {
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
			consumed[g] = true
			continue
		}
		for _, state := range stateList {
			key := recordKey(state)
			recordID := fmt.Sprintf("%s:%s", state.Type, state.Name)
			if scope.Matches("", "", "", "", state.Domain, recordID) {
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
	for g, cfgList := range restCfg {
		if consumed[g] {
			continue
		}
		for _, cfg := range cfgList {
			key := recordKey(cfg)
			recordID := fmt.Sprintf("%s:%s", cfg.Type, cfg.Name)
			if scope.Matches("", "", "", "", cfg.Domain, recordID) {
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
	return a.Domain == b.Domain && a.Type == b.Type && a.Name == b.Name && a.Value == b.Value && a.TTL == b.TTL && a.EffectiveLine() == b.EffectiveLine()
}
