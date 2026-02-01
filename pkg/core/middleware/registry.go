// Package middleware provides the core middleware system for request processing.
package middleware

import (
	"fmt"
	"sync"

	"github.com/ksysoev/opengate/pkg/core/policy"
)

// Registry manages middleware factories and creates middleware instances from policies.
// It is safe for concurrent use.
type Registry struct {
	factories map[string]MiddlewareFactory
	mu        sync.RWMutex
}

// NewRegistry creates a new middleware registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]MiddlewareFactory),
	}
}

// Register adds a middleware factory to the registry.
// Returns an error if a factory with the same type is already registered.
func (r *Registry) Register(factory MiddlewareFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	factoryType := factory.Type()
	if _, exists := r.factories[factoryType]; exists {
		return fmt.Errorf("middleware factory for type %q already registered", factoryType)
	}

	r.factories[factoryType] = factory

	return nil
}

// CreateMiddleware creates a middleware instance from a policy.
// Returns an error if the policy type is not registered or if middleware creation fails.
func (r *Registry) CreateMiddleware(p policy.Policy) (Middleware, error) {
	r.mu.RLock()
	factory, exists := r.factories[p.Type]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no middleware factory registered for type %q", p.Type)
	}

	middleware, err := factory.Create(p.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create middleware for policy %q: %w", p.Name, err)
	}

	return middleware, nil
}

// HasFactory returns true if a factory for the given type is registered.
func (r *Registry) HasFactory(middlewareType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[middlewareType]

	return exists
}
