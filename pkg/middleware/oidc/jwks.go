// Package oidc provides OIDC JWT validation middleware.
package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/ksysoev/opengate/pkg/core/request"
)

// JWKSCache manages lazy caching of JWKS keys with TTL-based expiration.
type JWKSCache struct {
	expires time.Time
	keys    map[string]*rsa.PublicKey
	runtime middleware.Runtime
	uri     string
	ttl     time.Duration
	mu      sync.RWMutex
}

// NewJWKSCache creates a new JWKS cache with the given URI, TTL, and runtime.
func NewJWKSCache(uri string, ttl time.Duration, runtime middleware.Runtime) *JWKSCache {
	if ttl == 0 {
		ttl = time.Hour // Default to 1 hour
	}

	return &JWKSCache{
		uri:     uri,
		ttl:     ttl,
		keys:    make(map[string]*rsa.PublicKey),
		runtime: runtime,
	}
}

// GetKey retrieves a key by kid. If the cache is expired or key not found, refreshes from JWKS URI.
func (c *JWKSCache) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Try to get from cache first
	c.mu.RLock()

	if time.Now().Before(c.expires) {
		if key, ok := c.keys[kid]; ok {
			c.mu.RUnlock()
			return key, nil
		}
	}

	c.mu.RUnlock()

	// Cache expired or key not found, refresh
	return c.refresh(ctx, kid)
}

// refresh fetches JWKS from the URI and updates the cache.
func (c *JWKSCache) refresh(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine might have refreshed while we were waiting
	if time.Now().Before(c.expires) {
		if key, ok := c.keys[kid]; ok {
			return key, nil
		}
	}

	// Fetch JWKS using runtime
	jwksURL, err := url.Parse(c.uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWKS URI: %w", err)
	}

	req := &request.Request{
		Method:  http.MethodGet,
		URL:     jwksURL,
		Headers: http.Header{},
		Body:    http.NoBody,
	}

	resp, err := c.runtime.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	// Parse JWKS
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Parse keys
	newKeys := make(map[string]*rsa.PublicKey)
	failedCount := 0

	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue // Only support RSA keys
		}

		// Decode N (modulus)
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			slog.Warn("Failed to decode RSA modulus (N) from JWKS", "kid", jwk.Kid, "error", err)

			failedCount++

			continue
		}

		// Decode E (exponent)
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			slog.Warn("Failed to decode RSA exponent (E) from JWKS", "kid", jwk.Kid, "error", err)

			failedCount++

			continue
		}

		// Validate exponent size to prevent overflow
		eBig := new(big.Int).SetBytes(eBytes)
		if !eBig.IsInt64() {
			slog.Warn("RSA exponent too large to fit in int64, skipping key", "kid", jwk.Kid)

			failedCount++

			continue
		}

		e64 := eBig.Int64()
		// Check if exponent fits in int on this platform
		maxInt := int64(^uint(0) >> 1)

		if e64 <= 0 || e64 > maxInt {
			slog.Warn("RSA exponent out of valid range for int, skipping key", "kid", jwk.Kid, "exponent", e64)

			failedCount++

			continue
		}

		// Create public key
		pubKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(e64),
		}

		newKeys[jwk.Kid] = pubKey
	}

	if len(newKeys) == 0 && len(jwks.Keys) > 0 {
		return nil, fmt.Errorf("failed to parse any keys from JWKS (%d keys failed)", failedCount)
	}

	if failedCount > 0 {
		slog.Warn("Some JWKS keys failed to parse", "failed", failedCount, "succeeded", len(newKeys))
	}

	// Update cache
	c.keys = newKeys
	c.expires = time.Now().Add(c.ttl)

	// Return requested key
	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key with kid %q not found in JWKS", kid)
	}

	return key, nil
}
