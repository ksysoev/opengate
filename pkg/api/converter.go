package api

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ksysoev/opengate/pkg/core/request"
)

// httpToCore converts an HTTP request to a core request.
func httpToCore(r *http.Request, pathParams map[string]string) (*request.Request, error) {
	if r == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	return &request.Request{
		Method:      r.Method,
		Path:        r.URL.Path,
		PathParams:  pathParams,
		QueryParams: r.URL.Query(),
		Headers:     r.Header,
		Body:        r.Body,
		RemoteAddr:  r.RemoteAddr,
		TLS:         r.TLS != nil,
		Host:        r.Host,
	}, nil
}

// coreToHTTP writes a core response to an HTTP response writer.
func coreToHTTP(w http.ResponseWriter, resp *request.Response) error {
	if resp == nil {
		return fmt.Errorf("core response is nil")
	}

	// Copy headers
	for key, values := range resp.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy body if present
	if resp.Body != nil && resp.Body != http.NoBody {
		defer resp.Body.Close()

		if _, err := io.Copy(w, resp.Body); err != nil {
			return fmt.Errorf("failed to copy response body: %w", err)
		}
	}

	return nil
}
