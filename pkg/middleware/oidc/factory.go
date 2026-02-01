// Package oidc provides OIDC JWT validation middleware.
package oidc

import (
	"fmt"
	"net/http"

	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/mitchellh/mapstructure"
)

// Factory creates OIDC middleware instances.
type Factory struct {
	httpClient *http.Client
}

// NewFactory creates a new OIDC middleware factory with the provided HTTP client.
func NewFactory(httpClient *http.Client) *Factory {
	return &Factory{
		httpClient: httpClient,
	}
}

// Type returns the middleware type identifier.
func (f *Factory) Type() string {
	return "oidc"
}

// Create builds an OIDC middleware from configuration.
func (f *Factory) Create(rawConfig map[string]interface{}) (middleware.Middleware, error) {
	var config Config

	// Decode configuration using mapstructure
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &config,
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(rawConfig); err != nil {
		return nil, fmt.Errorf("failed to decode OIDC config: %w", err)
	}

	// Create middleware with shared HTTP client
	mw, err := NewMiddleware(&config, f.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC middleware: %w", err)
	}

	return mw, nil
}
