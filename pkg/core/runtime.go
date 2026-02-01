// Package core provides core service logic and interfaces.
package core

// Runtime holds shared providers and resources for handlers.
// This provides a clean way to pass multiple dependencies without
// changing handler signatures as new providers are added.
//
// The Runtime struct follows the dependency injection pattern and makes
// it easy to add new providers (cache, metrics, logging, etc.) in the future
// without breaking existing handler constructors.
type Runtime struct {
	// HTTP provides HTTP client functionality for making requests to backend services.
	// Used by handlers that need to forward requests to external APIs.
	HTTP HTTPClient

	// Future fields can be added here as needed:
	// Cache  CacheProvider
	// Metrics MetricsProvider
	// Logger  LoggerProvider
}
