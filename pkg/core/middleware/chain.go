// Package middleware provides the core middleware system for request processing.
package middleware

import (
	"context"
	"fmt"
	"sync"

	"github.com/ksysoev/opengate/pkg/core/policy"
	"github.com/ksysoev/opengate/pkg/core/request"
)

// Chain manages middleware instances and builds execution chains for routes.
type Chain struct {
	registry        *Registry
	policies        map[string]policy.Policy
	middlewareCache map[string]Middleware
	mu              sync.RWMutex
}

// NewChain creates a new middleware chain builder.
func NewChain(registry *Registry, policies map[string]policy.Policy) (*Chain, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}

	if policies == nil {
		policies = make(map[string]policy.Policy)
	}

	return &Chain{
		registry:        registry,
		policies:        policies,
		middlewareCache: make(map[string]Middleware),
	}, nil
}

// Build creates a HandlerFunc that executes the middleware chain for the given policy names.
// The policies are executed in order, with the final handler called last.
// If no policies are specified, returns the final handler directly.
func (c *Chain) Build(policyNames []string, finalHandler HandlerFunc) HandlerFunc {
	// No middleware, return handler directly
	if len(policyNames) == 0 {
		return finalHandler
	}

	// Build middleware list
	middlewares := make([]Middleware, 0, len(policyNames))
	for _, policyName := range policyNames {
		middleware, err := c.getOrCreateMiddleware(policyName)
		if err != nil {
			// Return error handler if middleware creation fails
			return func(ctx context.Context, req *request.Request) (*request.Response, error) {
				return nil, fmt.Errorf("failed to build middleware chain: %w", err)
			}
		}

		middlewares = append(middlewares, middleware)
	}

	// Build the chain from right to left (last middleware wraps the handler)
	handler := finalHandler

	for i := len(middlewares) - 1; i >= 0; i-- {
		middleware := middlewares[i]
		next := handler

		handler = func(ctx context.Context, req *request.Request) (*request.Response, error) {
			return middleware.Process(ctx, req, next)
		}
	}

	return handler
}

// getOrCreateMiddleware retrieves a middleware from cache or creates it if not cached.
func (c *Chain) getOrCreateMiddleware(policyName string) (Middleware, error) {
	// Check cache first with read lock
	c.mu.RLock()
	middleware, ok := c.middlewareCache[policyName]
	c.mu.RUnlock()

	if ok {
		return middleware, nil
	}

	// Not in cache, acquire write lock to create
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine might have created it while we waited
	if middleware, ok := c.middlewareCache[policyName]; ok {
		return middleware, nil
	}

	// Get policy
	pol, ok := c.policies[policyName]
	if !ok {
		return nil, fmt.Errorf("policy %q not found", policyName)
	}

	// Create middleware
	middleware, err := c.registry.CreateMiddleware(pol)
	if err != nil {
		return nil, err
	}

	// Cache it
	c.middlewareCache[policyName] = middleware

	return middleware, nil
}

// ValidatePolicies checks that all policy names reference existing policies
// and that their middleware types are registered.
// Returns an error describing the first validation failure found.
func (c *Chain) ValidatePolicies(policyNames []string) error {
	for _, policyName := range policyNames {
		pol, ok := c.policies[policyName]
		if !ok {
			return fmt.Errorf("policy %q not found", policyName)
		}

		if !c.registry.HasFactory(pol.Type) {
			return fmt.Errorf("no middleware factory registered for type %q (policy %q)", pol.Type, policyName)
		}
	}

	return nil
}

// PreloadPolicies eagerly creates middleware instances for all specified policies.
// This validates policy configurations at startup rather than on first request.
// Returns an error if any policy fails to create its middleware.
func (c *Chain) PreloadPolicies(policyNames []string) error {
	for _, policyName := range policyNames {
		if _, err := c.getOrCreateMiddleware(policyName); err != nil {
			return fmt.Errorf("failed to preload policy %q: %w", policyName, err)
		}
	}

	return nil
}
