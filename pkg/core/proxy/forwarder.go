package proxy

import (
	"context"
	"errors"
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
type Forwarder struct {
	runtime *core.Runtime
}

// New creates a new proxy Forwarder instance.
func New(runtime *core.Runtime) *Forwarder {
	return &Forwarder{
		runtime: runtime,
	}
}

// Handle implements core.Handler interface for forwarding requests.
func (f *Forwarder) Handle(ctx context.Context, req *request.Request, rt *route.Route) (*request.Response, error) {
	// Validate route configuration
	if rt.Handler.BaseURL == "" {
		return nil, fmt.Errorf("%w: no backend URL configured", core.ErrInvalidRoute)
	}

	// Build backend URL
	backendURL, err := f.buildBackendURL(rt.Handler.BaseURL, req.Path, req.QueryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build backend URL: %w", err)
	}

	// Create backend request
	backendReq, err := f.createProxyRequest(ctx, req, backendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Execute request
	//nolint:bodyclose // Response body is passed to caller who is responsible for closing it
	resp, err := f.runtime.HTTP.Do(backendReq)
	if err != nil {
		// Check if error is timeout-related
		if f.isTimeoutError(err) {
			return nil, &core.BackendError{
				Err:        fmt.Errorf("%w: %v", core.ErrBackendTimeout, err),
				BackendURL: backendURL,
			}
		}

		return nil, &core.BackendError{
			Err:        fmt.Errorf("%w: %v", core.ErrBackendFailed, err),
			BackendURL: backendURL,
		}
	}

	// Convert to core.Response
	// NOTE: The caller is responsible for closing resp.Body
	return &request.Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       resp.Body,
	}, nil
}

// buildBackendURL constructs the full backend URL.
func (f *Forwarder) buildBackendURL(baseURL, reqPath string, queryParams url.Values) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Combine base URL with request path and query
	backendURL := *base
	backendURL.Path = strings.TrimSuffix(base.Path, "/") + reqPath
	backendURL.RawQuery = queryParams.Encode()

	return backendURL.String(), nil
}

// createProxyRequest creates a new HTTP request for the backend.
func (f *Forwarder) createProxyRequest(ctx context.Context, req *request.Request, backendURL string) (*http.Request, error) {
	proxyReq, err := http.NewRequestWithContext(ctx, req.Method, backendURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers
	f.copyHeaders(proxyReq.Header, req.Headers)

	// Set X-Forwarded headers
	f.setForwardedHeaders(proxyReq, req)

	return proxyReq, nil
}

// copyHeaders copies headers from source to destination.
func (f *Forwarder) copyHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if f.isHopByHopHeader(key) {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// isHopByHopHeader checks if a header is a hop-by-hop header.
func (f *Forwarder) isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, hopHeader := range hopByHopHeaders {
		if strings.EqualFold(hopHeader, header) {
			return true
		}
	}

	return false
}

// setForwardedHeaders sets X-Forwarded-* headers.
func (f *Forwarder) setForwardedHeaders(proxyReq *http.Request, req *request.Request) {
	// Use RemoteAddr as the source of truth to prevent IP spoofing
	clientIP := f.extractClientIP(req.RemoteAddr)

	// Append to existing X-Forwarded-For if present
	if xff := req.Headers.Get("X-Forwarded-For"); xff != "" {
		proxyReq.Header.Set("X-Forwarded-For", xff+", "+clientIP)
	} else {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	}

	if req.TLS {
		proxyReq.Header.Set("X-Forwarded-Proto", "https")
	} else {
		proxyReq.Header.Set("X-Forwarded-Proto", "http")
	}

	if req.Host != "" {
		proxyReq.Header.Set("X-Forwarded-Host", req.Host)
	}
}

// extractClientIP extracts the IP address from RemoteAddr.
func (f *Forwarder) extractClientIP(remoteAddr string) string {
	// RemoteAddr is in format "IP:port"
	if idx := strings.LastIndex(remoteAddr, ":"); idx > 0 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// isTimeoutError checks if an error is timeout-related.
func (f *Forwarder) isTimeoutError(err error) bool {
	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for net.Error with Timeout() == true
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}
