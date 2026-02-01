package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
)

// hopByHopHeaders is a map of HTTP headers that should not be forwarded through a proxy.
// These headers are connection-specific and apply only to the immediate connection.
var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailers":            {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

// SendRequest converts a core request to HTTP, executes it, and converts the response back.
// All HTTP-specific logic (header handling, filtering hop-by-hop headers, etc.) is encapsulated here.
//
// The request must have its URL field populated with the complete target URL.
// Returns a core response or an error wrapped with appropriate error types.
func (c *Client) SendRequest(ctx context.Context, req *request.Request) (*request.Response, error) {
	// Validate request URL
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if req.URL == nil {
		return nil, fmt.Errorf("request URL is nil")
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Copy headers (filtering hop-by-hop headers)
	c.copyHeaders(httpReq.Header, req.Headers)

	// Execute the HTTP request
	//nolint:bodyclose // Response body is passed to caller who is responsible for closing it
	resp, err := c.client.Do(httpReq)
	if err != nil {
		// Check if error is timeout-related
		if c.isTimeoutError(err) {
			return nil, &core.BackendError{
				Err:        fmt.Errorf("%w: %v", core.ErrBackendTimeout, err),
				BackendURL: req.URL.String(),
			}
		}

		return nil, &core.BackendError{
			Err:        fmt.Errorf("%w: %v", core.ErrBackendFailed, err),
			BackendURL: req.URL.String(),
		}
	}

	// Convert HTTP response to core response
	// NOTE: The caller is responsible for closing resp.Body
	return &request.Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       resp.Body,
	}, nil
}

// copyHeaders copies headers from source to destination, filtering hop-by-hop headers.
func (c *Client) copyHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if c.isHopByHopHeader(key) {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// isHopByHopHeader checks if a header is a hop-by-hop header that should not be forwarded.
// These headers are connection-specific and should not be proxied.
// Uses O(1) map lookup for performance.
func (c *Client) isHopByHopHeader(header string) bool {
	_, ok := hopByHopHeaders[strings.ToLower(header)]
	return ok
}

// isTimeoutError checks if an error is timeout-related.
// This distinguishes timeout errors from other network errors for better error handling.
func (c *Client) isTimeoutError(err error) bool {
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
