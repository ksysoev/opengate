// Package oidc provides OIDC JWT validation middleware.
package oidc

import (
	"fmt"

	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/mitchellh/mapstructure"
)

func init() {
	if err := middleware.Register("oidc", Create); err != nil {
		panic(fmt.Sprintf("failed to register oidc middleware: %v", err))
	}
}

// Create is the factory function for OIDC middleware.
// It decodes the configuration and creates a new OIDC middleware instance.
func Create(runtime middleware.Runtime, rawConfig map[string]any) (middleware.Middleware, error) {
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

	// Create middleware with runtime
	mw, err := NewMiddleware(runtime, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC middleware: %w", err)
	}

	return mw, nil
}
