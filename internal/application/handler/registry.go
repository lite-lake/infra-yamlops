package handler

import "sync"

type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

func (r *Registry) Register(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[h.EntityType()] = h
}

func (r *Registry) Get(entityType string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[entityType]
	return h, ok
}

func (r *Registry) All() map[string]Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Handler, len(r.handlers))
	for k, v := range r.handlers {
		result[k] = v
	}
	return result
}
