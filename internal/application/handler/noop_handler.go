package handler

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type NoopHandler struct {
	entityType string
}

func NewNoopHandler(entityType string) *NoopHandler {
	return &NoopHandler{entityType: entityType}
}

func (h *NoopHandler) EntityType() string {
	return h.entityType
}

func (h *NoopHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	return &Result{
		Change:  change,
		Success: true,
		Output:  "skipped (not a deployable entity)",
	}, nil
}
