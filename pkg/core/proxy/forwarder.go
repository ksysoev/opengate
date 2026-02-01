package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
)

// Forwarder handles forwarding HTTP requests to backend services.
// It builds the target URL and delegates actual HTTP communication to the runtime provider.
type Forwarder struct {
	runtime core.Runtime
}

// New creates a new proxy Forwarder instance.
// Returns an error if runtime is nil.
func New(runtime core.Runtime) (*Forwarder, error) {
	if runtime == nil {
		return nil, fmt.Errorf("%w: runtime cannot be nil", core.ErrInvalidRuntime)
	}

	return &Forwarder{
		runtime: runtime,
	}, nil
}

// Handle implements core.Handler interface for forwarding requests.
// It constructs the complete backend URL and delegates the HTTP request to the runtime.
func (f *Forwarder) Handle(ctx context.Context, req *request.Request, rt *route.Route) (*request.Response, error) {
	// Validate route configuration
	if rt.Handler.BaseURL == "" {
		return nil, fmt.Errorf("%w: no backend URL configured", core.ErrInvalidRoute)
	}

	// Build complete backend URL
	backendURL, err := f.buildBackendURL(rt.Handler.BaseURL, req.Path, req.QueryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build backend URL: %w", err)
	}

	// Set the target URL in the request
	req.URL = backendURL

	// Set X-Forwarded-* headers for proxy behavior
	f.setForwardedHeaders(req)

	// Delegate to runtime provider for actual HTTP communication
	return f.runtime.SendRequest(ctx, req)
}

// buildBackendURL constructs the complete backend URL by combining base URL, path, and query parameters.
// Query parameters from both the base URL and request are merged, with request parameters taking precedence.
func (f *Forwarder) buildBackendURL(baseURL, reqPath string, queryParams url.Values) (*url.URL, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Combine base URL path with request path
	// Trim trailing slash from base to avoid double slashes
	combinedPath := strings.TrimSuffix(base.Path, "/") + reqPath

	// Merge query parameters: start with base URL params, then add/override with request params
	mergedQuery := base.Query()
	for key, values := range queryParams {
		mergedQuery[key] = values
	}

	// Create new URL with combined path and merged query
	backendURL := *base
	backendURL.Path = combinedPath
	backendURL.RawQuery = mergedQuery.Encode()

	return &backendURL, nil
}

// setForwardedHeaders sets X-Forwarded-* headers to preserve client information.
// This is standard proxy behavior to inform backend services of the original request.
func (f *Forwarder) setForwardedHeaders(req *request.Request) {
	// Initialize headers if nil
	if req.Headers == nil {
		req.Headers = make(http.Header)
	}

	// Extract client IP from RemoteAddr to prevent IP spoofing
	clientIP := extractClientIP(req.RemoteAddr)

	// Append to existing X-Forwarded-For if present
	if xff := req.Headers.Get("X-Forwarded-For"); xff != "" {
		req.Headers.Set("X-Forwarded-For", xff+", "+clientIP)
	} else {
		req.Headers.Set("X-Forwarded-For", clientIP)
	}

	// Set protocol (http vs https)
	if req.TLS {
		req.Headers.Set("X-Forwarded-Proto", "https")
	} else {
		req.Headers.Set("X-Forwarded-Proto", "http")
	}

	// Set original host
	if req.Host != "" {
		req.Headers.Set("X-Forwarded-Host", req.Host)
	}
}

// extractClientIP extracts the IP address from RemoteAddr.
// Properly handles both IPv4 (192.168.1.1:8080) and IPv6 ([2001:db8::1]:8080) formats.
func extractClientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If SplitHostPort fails, return the original address
		// This handles cases where there's no port
		return remoteAddr
	}

	return host
}
