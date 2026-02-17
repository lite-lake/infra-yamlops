package handler

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type RegistryHandler struct{}

func NewRegistryHandler() *RegistryHandler {
	return &RegistryHandler{}
}

func (h *RegistryHandler) EntityType() string {
	return "registry"
}

func (h *RegistryHandler) Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error) {
	return &Result{
		Change:  change,
		Success: true,
		Output:  "skipped (not a deployable entity)",
	}, nil
}
