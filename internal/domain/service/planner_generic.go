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
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeDelete,
					Entity:   entityName,
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete %s %s", entityName, name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := stateMap[name]; exists {
			if !equals(state, cfg) {
				if scopeMatcher(name) {
					plan.AddChange(&valueobject.Change{
						Type:     valueobject.ChangeTypeUpdate,
						Entity:   entityName,
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update %s %s", entityName, name)},
					})
				}
			}
		} else {
			if scopeMatcher(name) {
				plan.AddChange(&valueobject.Change{
					Type:     valueobject.ChangeTypeCreate,
					Entity:   entityName,
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create %s %s", entityName, name)},
				})
			}
		}
	}
}

func alwaysMatchScope(_ string) bool {
	return true
}
