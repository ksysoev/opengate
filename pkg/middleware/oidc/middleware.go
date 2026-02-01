// Package oidc provides OIDC JWT validation middleware.
package oidc

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/ksysoev/opengate/pkg/core/request"
)

// Middleware performs OIDC JWT token validation.
type Middleware struct {
	jwksCache *JWKSCache
	config    Config
}

// NewMiddleware creates a new OIDC middleware instance.
// It uses the provided runtime for JWKS fetching.
func NewMiddleware(runtime middleware.Runtime, config *Config) (*Middleware, error) {
	if config.Issuer == "" {
		return nil, fmt.Errorf("issuer must be specified")
	}

	if config.Audience == "" {
		return nil, fmt.Errorf("audience must be specified")
	}

	if config.JWKSURI == "" {
		return nil, fmt.Errorf("jwks_uri must be specified")
	}

	return &Middleware{
		config:    *config,
		jwksCache: NewJWKSCache(config.JWKSURI, config.JWKSCacheTTL, runtime),
	}, nil
}

// Process validates the JWT token and passes the request to the next handler.
func (m *Middleware) Process(ctx context.Context, req *request.Request, next middleware.HandlerFunc) (*request.Response, error) {
	// Extract token from Authorization header
	token, err := m.extractToken(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", core.ErrUnauthorized, err.Error())
	}

	// Parse and validate token
	claims, err := m.validateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", core.ErrUnauthorized, err.Error())
	}

	// Check required claims
	if err := m.checkRequiredClaims(claims); err != nil {
		return nil, fmt.Errorf("%w: %s", core.ErrForbidden, err.Error())
	}

	// Map claims to headers
	m.mapClaimsToHeaders(req, claims)

	// Continue chain
	return next(ctx, req)
}

// extractToken extracts the JWT token from the Authorization header.
func (m *Middleware) extractToken(req *request.Request) (string, error) {
	authHeader := req.Headers.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header missing")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]
	if token == "" {
		return "", fmt.Errorf("token is empty")
	}

	// Validate token length to prevent DoS attacks with extremely large tokens
	// JWT tokens are typically under 8KB; 16KB is a generous upper limit
	const maxTokenLength = 16384
	if len(token) > maxTokenLength {
		return "", fmt.Errorf("token exceeds maximum length of %d bytes", maxTokenLength)
	}

	return token, nil
}

// validateToken parses and validates the JWT token.
func (m *Middleware) validateToken(ctx context.Context, tokenString string) (jwt.MapClaims, error) {
	// Parse token with explicit claims and expiration requirement
	token, err := jwt.ParseWithClaims(
		tokenString,
		jwt.MapClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Get kid from header
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("kid not found in token header")
			}

			// Fetch public key from JWKS
			return m.jwksCache.GetKey(ctx, kid)
		},
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	iss, err := claims.GetIssuer()
	if err != nil || iss != m.config.Issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Validate audience
	aud, err := claims.GetAudience()
	if err != nil {
		return nil, fmt.Errorf("invalid audience")
	}

	audienceValid := false

	for _, a := range aud {
		if a == m.config.Audience {
			audienceValid = true

			break
		}
	}

	if !audienceValid {
		return nil, fmt.Errorf("invalid audience")
	}

	return claims, nil
}

// checkRequiredClaims verifies that all required claims are present.
func (m *Middleware) checkRequiredClaims(claims jwt.MapClaims) error {
	for _, claimName := range m.config.RequiredClaims {
		if _, ok := claims[claimName]; !ok {
			return fmt.Errorf("required claim %q missing", claimName)
		}
	}

	return nil
}

// mapClaimsToHeaders maps JWT claims to HTTP headers.
func (m *Middleware) mapClaimsToHeaders(req *request.Request, claims jwt.MapClaims) {
	for _, mapping := range m.config.ClaimsToHeaders {
		if value, ok := claims[mapping.Claim]; ok {
			// Convert claim value to string based on type
			var strValue string

			switch v := value.(type) {
			case string:
				strValue = v
			case float64, int, int64, bool:
				strValue = fmt.Sprintf("%v", v)
			default:
				// Skip complex types (slices, maps, objects) - they're not suitable for headers
				continue
			}

			req.Headers.Set(mapping.Header, strValue)
		}
	}
}
