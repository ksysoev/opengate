package proxy

import (
	"context"
	"fmt"
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

	// Delegate to runtime provider for actual HTTP communication
	return f.runtime.SendRequest(ctx, req)
}

// buildBackendURL constructs the complete backend URL by combining base URL, path, and query parameters.
// This ensures proper URL construction with correct path joining and query encoding.
func (f *Forwarder) buildBackendURL(baseURL, reqPath string, queryParams url.Values) (*url.URL, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Combine base URL path with request path
	// Trim trailing slash from base to avoid double slashes
	combinedPath := strings.TrimSuffix(base.Path, "/") + reqPath

	// Create new URL with combined path and query
	backendURL := *base
	backendURL.Path = combinedPath
	backendURL.RawQuery = queryParams.Encode()

	return &backendURL, nil
}
