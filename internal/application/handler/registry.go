package handler

type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

func (r *Registry) Register(h Handler) {
	r.handlers[h.EntityType()] = h
}

func (r *Registry) Get(entityType string) (Handler, bool) {
	h, ok := r.handlers[entityType]
	return h, ok
}

func (r *Registry) All() map[string]Handler {
	result := make(map[string]Handler, len(r.handlers))
	for k, v := range r.handlers {
		result[k] = v
	}
	return result
}
