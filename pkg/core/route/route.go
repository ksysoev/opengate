// Package route defines core routing types for the OpenGate API gateway.
package route

// Route represents a single API route configuration.
type Route struct {
	Path        string
	Method      string
	OperationID string
	Handler     Handler
}

// Handler contains the backend routing configuration for a route.
type Handler struct {
	Type       string
	BaseURL    string
	Location   string
	StatusCode int
}
