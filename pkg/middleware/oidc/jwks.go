// Package oidc provides OIDC JWT validation middleware.
package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWKSCache manages lazy caching of JWKS keys with TTL-based expiration.
type JWKSCache struct {
	expires time.Time
	keys    map[string]*rsa.PublicKey
	client  *http.Client
	uri     string
	ttl     time.Duration
	mu      sync.RWMutex
}

// NewJWKSCache creates a new JWKS cache with the given URI and TTL.
func NewJWKSCache(uri string, ttl time.Duration) *JWKSCache {
	if ttl == 0 {
		ttl = time.Hour // Default to 1 hour
	}

	return &JWKSCache{
		uri:    uri,
		ttl:    ttl,
		keys:   make(map[string]*rsa.PublicKey),
		client: &http.Client{Timeout: 10 * time.Second},
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

	// Fetch JWKS
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.uri, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS request: %w", err)
	}

	resp, err := c.client.Do(req)
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

	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue // Only support RSA keys
		}

		// Decode N (modulus)
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}

		// Decode E (exponent)
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}

		// Create public key
		pubKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		}

		newKeys[jwk.Kid] = pubKey
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
