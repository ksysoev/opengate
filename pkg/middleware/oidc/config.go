// Package oidc provides OIDC JWT validation middleware.
package oidc

import "time"

// Config holds configuration for OIDC middleware.
type Config struct {
	Issuer          string          `mapstructure:"issuer"`
	Audience        string          `mapstructure:"audience"`
	JWKSURI         string          `mapstructure:"jwks_uri"`
	ClaimsToHeaders []ClaimToHeader `mapstructure:"claims_to_headers"`
	RequiredClaims  []string        `mapstructure:"required_claims"`
	JWKSCacheTTL    time.Duration   `mapstructure:"jwks_cache_ttl"`
}

// ClaimToHeader defines a mapping from JWT claim to HTTP header.
type ClaimToHeader struct {
	// Claim is the name of the JWT claim
	Claim string `mapstructure:"claim"`

	// Header is the HTTP header name to set
	Header string `mapstructure:"header"`
}
