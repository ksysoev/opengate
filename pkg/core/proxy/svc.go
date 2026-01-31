// Package proxy provides HTTP proxying functionality for the API gateway.
package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ksysoev/opengate/pkg/core/router"
)

const (
	defaultTimeout = 30 * time.Second
)

// Handler handles proxying HTTP requests to backend services.
type Handler struct {
	client  *http.Client
	timeout time.Duration
}

// New creates a new proxy Handler instance.
func New() *Handler {
	return &Handler{
		client: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		timeout: defaultTimeout,
	}
}

// NewWithTimeout creates a new proxy Handler with a custom timeout.
func NewWithTimeout(timeout time.Duration) *Handler {
	h := New()
	h.timeout = timeout
	h.client.Timeout = timeout

	return h
}

// ServeHTTP implements http.Handler interface for proxying requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := router.GetRoute(r.Context())
	if route == nil {
		slog.ErrorContext(r.Context(), "No route found in context")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if route.Handler.BaseURL == "" {
		slog.ErrorContext(r.Context(), "No backend URL configured for route",
			"path", route.Path, "method", route.Method)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if err := h.proxyRequest(w, r, route.Handler.BaseURL); err != nil {
		slog.ErrorContext(r.Context(), "Failed to proxy request",
			"error", err, "backend_url", route.Handler.BaseURL)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)

		return
	}
}

// proxyRequest forwards the request to the backend service.
func (h *Handler) proxyRequest(w http.ResponseWriter, r *http.Request, baseURL string) error {
	backendURL, err := h.buildBackendURL(baseURL, r.URL)
	if err != nil {
		return fmt.Errorf("failed to build backend URL: %w", err)
	}

	proxyReq, err := h.createProxyRequest(r, backendURL)
	if err != nil {
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	resp, err := h.client.Do(proxyReq)
	if err != nil {
		return fmt.Errorf("failed to send request to backend: %w", err)
	}
	defer resp.Body.Close()

	return h.copyResponse(w, resp)
}

// buildBackendURL constructs the full backend URL.
func (h *Handler) buildBackendURL(baseURL string, reqURL *url.URL) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Combine base URL with request path and query
	backendURL := *base
	backendURL.Path = strings.TrimSuffix(base.Path, "/") + reqURL.Path
	backendURL.RawQuery = reqURL.RawQuery

	return backendURL.String(), nil
}

// createProxyRequest creates a new HTTP request for the backend.
func (h *Handler) createProxyRequest(r *http.Request, backendURL string) (*http.Request, error) {
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, backendURL, r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers
	h.copyHeaders(proxyReq.Header, r.Header)

	// Set X-Forwarded headers
	h.setForwardedHeaders(proxyReq, r)

	return proxyReq, nil
}

// copyHeaders copies headers from source to destination.
func (h *Handler) copyHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if h.isHopByHopHeader(key) {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// isHopByHopHeader checks if a header is a hop-by-hop header.
func (h *Handler) isHopByHopHeader(header string) bool {
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
func (h *Handler) setForwardedHeaders(proxyReq, originalReq *http.Request) {
	// Use RemoteAddr as the source of truth to prevent IP spoofing
	clientIP := h.extractClientIP(originalReq.RemoteAddr)

	// Append to existing X-Forwarded-For if present
	if xff := originalReq.Header.Get("X-Forwarded-For"); xff != "" {
		proxyReq.Header.Set("X-Forwarded-For", xff+", "+clientIP)
	} else {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	}

	if originalReq.TLS != nil {
		proxyReq.Header.Set("X-Forwarded-Proto", "https")
	} else {
		proxyReq.Header.Set("X-Forwarded-Proto", "http")
	}

	if originalReq.Host != "" {
		proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
	}
}

// extractClientIP extracts the IP address from RemoteAddr.
func (h *Handler) extractClientIP(remoteAddr string) string {
	// RemoteAddr is in format "IP:port"
	if idx := strings.LastIndex(remoteAddr, ":"); idx > 0 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// copyResponse copies the backend response to the client.
func (h *Handler) copyResponse(w http.ResponseWriter, resp *http.Response) error {
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	return nil
}
