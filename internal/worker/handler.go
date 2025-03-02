// package worker - job processing worker
package worker

import (
	"context"
)

// Handler is the interface for job handlers.
type Handler interface {
	// Handle processes a job and returns the result or error.
	Handle(ctx context.Context, payload []byte) ([]byte, error)
}

// HandlerFunc is an adapter to allow ordinary functions to be used as handlers.
type HandlerFunc func(ctx context.Context, payload []byte) ([]byte, error)

// Handle calls f(ctx, payload).
func (f HandlerFunc) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	return f(ctx, payload)
}

// Registry holds registered job handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register registers a handler for a job type.
func (r *Registry) Register(jobType string, handler Handler) {
	r.handlers[jobType] = handler
}

// RegisterFunc registers a handler function for a job type.
func (r *Registry) RegisterFunc(jobType string, handler HandlerFunc) {
	r.handlers[jobType] = handler
}

// Get returns the handler for a job type.
func (r *Registry) Get(jobType string) (Handler, bool) {
	h, ok := r.handlers[jobType]
	return h, ok
}

// Types returns all registered job types.
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}
