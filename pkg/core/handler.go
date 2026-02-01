// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"net/http"

	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
)

// Handler defines the interface for handling requests in the core layer.
// This is the main abstraction that decouples business logic from HTTP protocol.
// Implementations should be protocol-agnostic and work only with internal request types.
type Handler interface {
	// Handle processes a request and returns a response or error.
	// The route parameter contains routing configuration from OpenAPI spec.
	// Context is used for cancellation, timeouts, and request-scoped values.
	Handle(ctx context.Context, req *request.Request, rt *route.Route) (*request.Response, error)
}

// HTTPClient defines the interface for making HTTP requests.
// This interface is consumed by multiple handler types (e.g., forwarder).
// It abstracts the HTTP client implementation, allowing for easy testing and flexibility.
type HTTPClient interface {
	// Do executes an HTTP request and returns the response.
	// The caller is responsible for closing the response body.
	Do(req *http.Request) (*http.Response, error)
}
