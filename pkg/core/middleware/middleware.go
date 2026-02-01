// Package middleware provides the core middleware system for request processing.
package middleware

import (
	"context"

	"github.com/ksysoev/opengate/pkg/core/request"
)

// HandlerFunc is a function type that processes a request and returns a response.
// It represents the next step in the middleware chain or the final handler.
type HandlerFunc func(context.Context, *request.Request) (*request.Response, error)

// Middleware defines the interface for core-level middleware.
// Middleware operates on internal request types after route matching but before handler execution.
// This allows middleware to be protocol-agnostic and work only with business logic concerns.
type Middleware interface {
	// Process handles the request, with the ability to call next() to continue the chain.
	// Returning an error stops the chain and propagates the error to the caller.
	// The middleware can:
	// - Inspect/modify the request before calling next
	// - Inspect/modify the response after calling next
	// - Short-circuit by returning without calling next
	// - Wrap errors from next with additional context
	Process(ctx context.Context, req *request.Request, next HandlerFunc) (*request.Response, error)
}

// MiddlewareFactory creates middleware instances from configuration.
// Each middleware type (oidc, rate-limit, etc.) implements this interface.
type MiddlewareFactory interface {
	// Create builds a configured middleware instance from raw config map.
	// Returns an error if the configuration is invalid or incomplete.
	Create(config map[string]interface{}) (Middleware, error)

	// Type returns the middleware type identifier (e.g., "oidc", "rate-limit").
	// This is used to match policy types in config.yml to factory implementations.
	Type() string
}
