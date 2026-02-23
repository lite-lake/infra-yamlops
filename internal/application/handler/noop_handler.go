package handler

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

var NoopEntities = []string{"isp", "zone", "domain", "certificate"}

type HandlerRegistry interface {
	Register(h Handler)
}

func RegisterNoopHandlers(r HandlerRegistry) {
	for _, et := range NoopEntities {
		r.Register(NewNoopHandler(et))
	}
}

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
