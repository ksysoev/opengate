// Package request provides protocol-agnostic request and response types.
package request

import (
	"io"
	"net/http"
	"net/url"
)

// Request represents an internal request abstraction.
// This is protocol-agnostic and contains all necessary request data.
//
// Path is used for incoming request routing (matching against route patterns).
// URL is used for outbound requests (complete target URL constructed by handlers).
type Request struct {
	Body        io.ReadCloser
	PathParams  map[string]string
	QueryParams url.Values
	Headers     http.Header
	Method      string
	Path        string   // Incoming request path for routing
	URL         *url.URL // Complete target URL for outbound requests (set by handlers)
	RemoteAddr  string
	Host        string
	TLS         bool
}

// Response represents an internal response abstraction.
type Response struct {
	Body       io.ReadCloser
	Headers    http.Header
	StatusCode int
}
