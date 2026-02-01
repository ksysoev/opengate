// Package middleware provides the core middleware system for request processing.
package middleware

import (
	"fmt"
	"sync"

	"github.com/ksysoev/opengate/pkg/core/policy"
)

var (
	instance *Registry
	once     sync.Once
)

// Registry manages middleware factory functions as a singleton.
// It is safe for concurrent use.
type Registry struct {
	factories map[string]FactoryFunc
	mu        sync.RWMutex
}

// GetRegistry returns the singleton registry instance.
// The registry is lazily initialized on first call.
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			factories: make(map[string]FactoryFunc),
		}
	})

	return instance
}

// Register adds a middleware factory function to the global registry.
// Returns an error if a factory with the same name is already registered.
// This is a convenience function that calls GetRegistry().Register().
func Register(name string, factory FactoryFunc) error {
	return GetRegistry().Register(name, factory)
}

// Register adds a middleware factory function to the registry.
// Returns an error if a factory with the same name is already registered.
func (r *Registry) Register(name string, factory FactoryFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("middleware %q already registered", name)
	}

	r.factories[name] = factory

	return nil
}

// CreateMiddleware creates a middleware instance for the given policy.
// The runtime is passed to the factory function for making external calls.
// Returns an error if the policy type is not registered or if middleware creation fails.
func (r *Registry) CreateMiddleware(p policy.Policy, runtime Runtime) (Middleware, error) {
	r.mu.RLock()
	factory, exists := r.factories[p.Type]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no middleware factory registered for type %q", p.Type)
	}

	mw, err := factory(runtime, p.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create middleware for policy %q: %w", p.Name, err)
	}

	return mw, nil
}

// HasFactory returns true if a factory for the given type is registered.
func (r *Registry) HasFactory(middlewareType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[middlewareType]

	return exists
}

// resetRegistry resets the singleton for testing.
// This is intentionally unexported - only accessible from tests in the same package.
//
//nolint:unused // Only used in tests
func resetRegistry() {
	once = sync.Once{}
	instance = nil
}
