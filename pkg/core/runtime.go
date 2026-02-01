// Package core provides core service logic and interfaces.
package core

import (
	"context"

	"github.com/ksysoev/opengate/pkg/core/request"
)

// Runtime defines the interface for accessing shared providers and resources.
// This provides a clean way to pass multiple dependencies without
// changing handler signatures as new providers are added.
//
// Handlers should depend on this interface rather than concrete implementation,
// which simplifies testing and maintains clean architecture boundaries.
type Runtime interface {
	// SendRequest sends a request to the target URL specified in req.URL.
	// The request should have its URL field populated before calling this method.
	// Returns the response or an error if the request fails.
	SendRequest(ctx context.Context, req *request.Request) (*request.Response, error)

	// Future methods can be added here as needed:
	// GetCache() CacheProvider
	// GetMetrics() MetricsProvider
	// GetLogger() LoggerProvider
}

// HTTPProvider defines the interface for making HTTP requests.
// This abstracts the HTTP client implementation at the provider level.
type HTTPProvider interface {
	// SendRequest converts a core request to HTTP, executes it, and converts the response back.
	// All HTTP-specific logic (header handling, forwarding, etc.) is encapsulated here.
	SendRequest(ctx context.Context, req *request.Request) (*request.Response, error)
}

// RuntimeImpl is the concrete implementation of Runtime interface.
// It holds shared providers and resources for handlers.
type RuntimeImpl struct {
	httpProvider HTTPProvider
}

// NewRuntime creates a new Runtime implementation with the provided HTTP provider.
// Returns an error if the HTTP provider is nil.
func NewRuntime(httpProvider HTTPProvider) (Runtime, error) {
	if httpProvider == nil {
		return nil, ErrInvalidRuntime
	}

	return &RuntimeImpl{
		httpProvider: httpProvider,
	}, nil
}

// SendRequest delegates the request to the HTTP provider.
func (r *RuntimeImpl) SendRequest(ctx context.Context, req *request.Request) (*request.Response, error) {
	return r.httpProvider.SendRequest(ctx, req)
}
