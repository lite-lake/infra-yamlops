package service

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func planSimpleEntity[T any](
	plan *valueobject.Plan,
	cfgMap map[string]*T,
	stateMap map[string]*T,
	equals func(a, b *T) bool,
	entityName string,
	scopeMatcher func(name string) bool,
) {
	for name, state := range stateMap {
		if _, exists := cfgMap[name]; !exists {
			if scopeMatcher(name) {
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeDelete,
					entityName,
					name,
					state,
					nil,
					[]string{fmt.Sprintf("delete %s %s", entityName, name)},
					false,
				))
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := stateMap[name]; exists {
			if !equals(state, cfg) {
				if scopeMatcher(name) {
					plan.AddChange(valueobject.NewChangeFull(
						valueobject.ChangeTypeUpdate,
						entityName,
						name,
						state,
						cfg,
						[]string{fmt.Sprintf("update %s %s", entityName, name)},
						false,
					))
				}
			}
		} else {
			if scopeMatcher(name) {
				plan.AddChange(valueobject.NewChangeFull(
					valueobject.ChangeTypeCreate,
					entityName,
					name,
					nil,
					cfg,
					[]string{fmt.Sprintf("create %s %s", entityName, name)},
					false,
				))
			}
		}
	}
}
