package usecase

import (
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func ExtractFromChange[T any](ch *valueobject.Change) (*T, bool) {
	if ch.NewState() != nil {
		if val, ok := ch.NewState().(*T); ok {
			return val, true
		}
	}
	if ch.OldState() != nil {
		if val, ok := ch.OldState().(*T); ok {
			return val, true
		}
	}
	return nil, false
}
