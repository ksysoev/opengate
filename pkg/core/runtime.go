// Package core provides core service logic and interfaces.
package core

// Runtime defines the interface for accessing shared providers and resources.
// This provides a clean way to pass multiple dependencies without
// changing handler signatures as new providers are added.
//
// Handlers should depend on this interface rather than concrete implementation,
// which simplifies testing and maintains clean architecture boundaries.
type Runtime interface {
	// GetHTTPClient returns the HTTP client for making requests to backend services.
	GetHTTPClient() HTTPClient

	// Future methods can be added here as needed:
	// GetCache() CacheProvider
	// GetMetrics() MetricsProvider
	// GetLogger() LoggerProvider
}

// RuntimeImpl is the concrete implementation of Runtime interface.
// It holds shared providers and resources for handlers.
type RuntimeImpl struct {
	http HTTPClient
}

// NewRuntime creates a new Runtime implementation with the provided HTTP client.
// Returns an error if the HTTP client is nil.
func NewRuntime(httpClient HTTPClient) (Runtime, error) {
	if httpClient == nil {
		return nil, ErrInvalidRuntime
	}

	return &RuntimeImpl{
		http: httpClient,
	}, nil
}

// GetHTTPClient returns the HTTP client for making requests to backend services.
func (r *RuntimeImpl) GetHTTPClient() HTTPClient {
	return r.http
}
