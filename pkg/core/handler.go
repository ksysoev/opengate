// Package core provides core service logic and interfaces.
package core

import (
	"context"

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
