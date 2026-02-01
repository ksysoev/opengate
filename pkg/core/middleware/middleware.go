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

// Runtime provides capabilities that middlewares may need during request processing.
// This interface is satisfied by core.Runtime, avoiding import cycles.
type Runtime interface {
	// SendRequest sends a request to the target URL specified in req.URL.
	// Returns the response or an error if the request fails.
	SendRequest(ctx context.Context, req *request.Request) (*request.Response, error)
}

// FactoryFunc is the standard function signature for creating middlewares.
// It receives runtime for making external calls and raw configuration.
// Middleware implementations register factory functions via Register().
type FactoryFunc func(runtime Runtime, config map[string]any) (Middleware, error)
