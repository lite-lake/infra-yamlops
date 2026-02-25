package usecase

import (
	"github.com/litelake/yamlops/internal/application/handler"
)

type HandlerRegistry struct {
	registry handlerRegistry
}

type handlerRegistry interface {
	Register(h handler.Handler)
	Get(entityType string) (handler.Handler, bool)
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		registry: handler.NewRegistry(),
	}
}

func NewHandlerRegistryWithRegistry(r handlerRegistry) *HandlerRegistry {
	return &HandlerRegistry{registry: r}
}

func (r *HandlerRegistry) Register(h handler.Handler) {
	r.registry.Register(h)
}

func (r *HandlerRegistry) Get(entityType string) (handler.Handler, bool) {
	return r.registry.Get(entityType)
}

func (r *HandlerRegistry) RegisterDefaults() {
	defaultHandlers := []handler.Handler{
		handler.NewDNSHandler(),
		handler.NewServiceHandler(),
		handler.NewInfraServiceHandler(),
		handler.NewServerHandler(),
	}
	for _, h := range defaultHandlers {
		if _, ok := r.registry.Get(h.EntityType()); !ok {
			r.registry.Register(h)
		}
	}
	handler.RegisterNoopHandlers(r.registry)
}

func (r *HandlerRegistry) Registry() handlerRegistry {
	return r.registry
}
