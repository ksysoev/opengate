// Package oidc provides OIDC JWT validation middleware.
package oidc

import (
	"fmt"

	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/mitchellh/mapstructure"
)

// Factory creates OIDC middleware instances.
type Factory struct{}

// NewFactory creates a new OIDC middleware factory.
func NewFactory() *Factory {
	return &Factory{}
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

	// Create middleware
	mw, err := NewMiddleware(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC middleware: %w", err)
	}

	return mw, nil
}
