// Package middleware provides the core middleware system for request processing.
package middleware

import (
	"context"
	"fmt"
	"sync"

	"github.com/ksysoev/opengate/pkg/core/policy"
	"github.com/ksysoev/opengate/pkg/core/request"
)

var (
	instance *Registry
	once     sync.Once
)

// Registry manages middleware factory functions as a singleton.
// It is safe for concurrent use and caches middleware instances by policy name.
type Registry struct {
	factories       map[string]FactoryFunc
	middlewareCache map[string]Middleware // Cache by policy name
	runtime         Runtime
	policies        map[string]policy.Policy
	mu              sync.RWMutex
}

// GetRegistry returns the singleton registry instance.
// The registry is lazily initialized on first call.
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			factories:       make(map[string]FactoryFunc),
			middlewareCache: make(map[string]Middleware),
		}
	})

	return instance
}

// Register adds a middleware factory function to the global registry.
// Returns an error if a factory with the same name is already registered.
// This is a convenience function that calls GetRegistry().Register().
func Register(name string, factory FactoryFunc) error {
	if name == "" {
		return fmt.Errorf("middleware name cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	return GetRegistry().Register(name, factory)
}

// Register adds a middleware factory function to the registry.
// Returns an error if a factory with the same name is already registered.
func (r *Registry) Register(name string, factory FactoryFunc) error {
	if name == "" {
		return fmt.Errorf("middleware name cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("middleware %q already registered", name)
	}

	r.factories[name] = factory

	return nil
}

// Initialize sets up the registry with runtime and policies.
// This should be called once at application startup before handling requests.
func (r *Registry) Initialize(runtime Runtime, policies map[string]policy.Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if runtime == nil {
		return fmt.Errorf("runtime cannot be nil")
	}

	if policies == nil {
		policies = make(map[string]policy.Policy)
	}

	r.runtime = runtime
	r.policies = policies

	return nil
}

// Initialize is a convenience function that calls GetRegistry().Initialize().
func Initialize(runtime Runtime, policies map[string]policy.Policy) error {
	return GetRegistry().Initialize(runtime, policies)
}

// getOrCreateMiddleware retrieves a middleware from cache or creates it if not cached.
func (r *Registry) getOrCreateMiddleware(policyName string) (Middleware, error) {
	// Check cache first with read lock
	r.mu.RLock()
	middleware, ok := r.middlewareCache[policyName]
	runtime := r.runtime
	policies := r.policies
	r.mu.RUnlock()

	if ok {
		return middleware, nil
	}

	// Not in cache, acquire write lock to create
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check: another goroutine might have created it while we waited
	if middleware, ok := r.middlewareCache[policyName]; ok {
		return middleware, nil
	}

	// Get policy
	pol, ok := policies[policyName]
	if !ok {
		return nil, fmt.Errorf("policy %q not found", policyName)
	}

	// Get factory
	factory, exists := r.factories[pol.Type]
	if !exists {
		return nil, fmt.Errorf("no middleware factory registered for type %q", pol.Type)
	}

	// Create middleware
	middleware, err := factory(runtime, pol.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create middleware for policy %q: %w", pol.Name, err)
	}

	// Cache it
	r.middlewareCache[policyName] = middleware

	return middleware, nil
}

// BuildHandler wraps a handler with middleware chain based on policy names.
// The middlewares are executed in order, with the handler called last.
// Returns the original handler if no policies are specified.
func (r *Registry) BuildHandler(policyNames []string, handler HandlerFunc) HandlerFunc {
	if len(policyNames) == 0 {
		return handler
	}

	// Build middleware list
	middlewares := make([]Middleware, 0, len(policyNames))
	for _, policyName := range policyNames {
		middleware, err := r.getOrCreateMiddleware(policyName)
		if err != nil {
			// Return error handler if middleware creation fails
			return func(ctx context.Context, req *request.Request) (*request.Response, error) {
				return nil, fmt.Errorf("failed to build middleware chain: %w", err)
			}
		}

		middlewares = append(middlewares, middleware)
	}

	// Build the chain from right to left (last middleware wraps the handler)
	result := handler

	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		next := result

		result = func(ctx context.Context, req *request.Request) (*request.Response, error) {
			return mw.Process(ctx, req, next)
		}
	}

	return result
}

// BuildHandler is a convenience function that calls GetRegistry().BuildHandler().
func BuildHandler(policyNames []string, handler HandlerFunc) HandlerFunc {
	return GetRegistry().BuildHandler(policyNames, handler)
}

// ValidatePolicies checks that all policy names reference existing policies
// and that their middleware types are registered.
// Returns an error describing the first validation failure found.
func (r *Registry) ValidatePolicies(policyNames []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, policyName := range policyNames {
		pol, ok := r.policies[policyName]
		if !ok {
			return fmt.Errorf("policy %q not found", policyName)
		}

		if _, exists := r.factories[pol.Type]; !exists {
			return fmt.Errorf("no middleware factory registered for type %q (policy %q)", pol.Type, policyName)
		}
	}

	return nil
}

// ValidatePolicies is a convenience function that calls GetRegistry().ValidatePolicies().
func ValidatePolicies(policyNames []string) error {
	return GetRegistry().ValidatePolicies(policyNames)
}

// PreloadPolicies eagerly creates middleware instances for all specified policies.
// This validates policy configurations at startup rather than on first request.
// Returns an error if any policy fails to create its middleware.
func (r *Registry) PreloadPolicies(policyNames []string) error {
	for _, policyName := range policyNames {
		if _, err := r.getOrCreateMiddleware(policyName); err != nil {
			return fmt.Errorf("failed to preload policy %q: %w", policyName, err)
		}
	}

	return nil
}

// PreloadPolicies is a convenience function that calls GetRegistry().PreloadPolicies().
func PreloadPolicies(policyNames []string) error {
	return GetRegistry().PreloadPolicies(policyNames)
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
// WARNING: This function is not safe for concurrent use. Tests using it must ensure
// no concurrent access to the registry occurs during reset.
//
//nolint:unused // Only used in tests
func resetRegistry() {
	if instance != nil {
		instance.mu.Lock()
		defer instance.mu.Unlock()
	}

	once = sync.Once{}
	instance = nil
}
