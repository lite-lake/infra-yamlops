package handler

import (
	"context"
	"fmt"
	"strings"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/registry"
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

	switch change.Type() {
	case valueobject.ChangeTypeCreate, valueobject.ChangeTypeUpdate:
		return h.handleCreateOrUpdate(ctx, change, deps)
	case valueobject.ChangeTypeDelete:
		result.Success = true
		result.Output = "server removed"
		return result, nil
	default:
		result.Success = true
		result.Output = "no action needed"
		return result, nil
	}
}

func (h *ServerHandler) handleCreateOrUpdate(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	server := h.getServerFromChange(change)
	if server == nil {
		result.Error = fmt.Errorf("%w: %s", domainerr.ErrServerNotRegistered, change.Name())
		return result, nil
	}

	if len(server.Environment.Registries) == 0 {
		result.Success = true
		action := "updated"
		if change.Type() == valueobject.ChangeTypeCreate {
			action = "registered"
		}
		result.Output = fmt.Sprintf("server %s (no registries configured)", action)
		return result, nil
	}

	client, err := deps.SSHClient(change.Name())
	if err != nil {
		result.Error = fmt.Errorf("get SSH client: %w", err)
		return result, nil
	}

	regManager := registry.NewManager(client, deps.GetAllRegistries(), deps.Secrets())
	loginResults := make([]string, 0, len(server.Environment.Registries))
	hasErrors := false

	for _, regName := range server.Environment.Registries {
		loginResult, loginErr := regManager.EnsureLoggedIn(regName)
		if loginErr != nil || !loginResult.Success {
			hasErrors = true
			loginResults = append(loginResults, fmt.Sprintf("❌ %s: %s", regName, loginResult.Message))
			if loginResult.Error != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", regName, loginResult.Error))
			}
		} else {
			loginResults = append(loginResults, fmt.Sprintf("✅ %s: %s", regName, loginResult.Message))
		}
	}

	if hasErrors {
		result.Success = false
		result.Output = fmt.Sprintf("server %s with registry login issues:\n%s",
			map[bool]string{true: "registered", false: "updated"}[change.Type() == valueobject.ChangeTypeCreate],
			strings.Join(loginResults, "\n"))
	} else {
		result.Success = true
		result.Output = fmt.Sprintf("server %s and logged in to %d registries:\n%s",
			map[bool]string{true: "registered", false: "updated"}[change.Type() == valueobject.ChangeTypeCreate],
			len(loginResults),
			strings.Join(loginResults, "\n"))
	}

	return result, nil
}

func (h *ServerHandler) getServerFromChange(change *valueobject.Change) *entity.Server {
	if change.NewState() != nil {
		if server, ok := change.NewState().(*entity.Server); ok {
			return server
		}
	}
	if change.OldState() != nil {
		if server, ok := change.OldState().(*entity.Server); ok {
			return server
		}
	}
	return nil
}
