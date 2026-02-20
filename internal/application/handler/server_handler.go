package handler

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type ServerHandler struct{}

func NewServerHandler() *ServerHandler {
	return &ServerHandler{}
}

func (h *ServerHandler) EntityType() string {
	return "server"
}

func (h *ServerHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	switch change.Type {
	case valueobject.ChangeTypeCreate:
		result.Success = true
		result.Output = "server registered"
	case valueobject.ChangeTypeUpdate:
		result.Success = true
		result.Output = "server updated"
	case valueobject.ChangeTypeDelete:
		result.Success = true
		result.Output = "server removed"
	default:
		result.Success = true
		result.Output = "no action needed"
	}

	return result, nil
}
