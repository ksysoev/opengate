package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_SendRequest_Success(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request was received correctly
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "value1", r.URL.Query().Get("param1"))

		// Verify headers were copied
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-value", r.Header.Get("X-Custom-Header"))

		// Verify X-Forwarded headers were set
		assert.Contains(t, r.Header.Get("X-Forwarded-For"), "192.168.1.1")
		assert.Equal(t, "https", r.Header.Get("X-Forwarded-Proto"))
		assert.Equal(t, "example.com", r.Header.Get("X-Forwarded-Host"))

		// Verify hop-by-hop headers were filtered
		assert.Empty(t, r.Header.Get("Connection"))
		assert.Empty(t, r.Header.Get("Keep-Alive"))

		// Send response
		w.Header().Set("X-Response-Header", "response-value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response body"))
	}))
	defer server.Close()

	// Create client
	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	// Parse server URL
	serverURL, err := url.Parse(server.URL + "/test-path?param1=value1")
	require.NoError(t, err)

	// Create request
	coreReq := &request.Request{
		Method: "POST",
		URL:    serverURL,
		Headers: http.Header{
			"Content-Type":    []string{"application/json"},
			"X-Custom-Header": []string{"test-value"},
			"Connection":      []string{"keep-alive"}, // Should be filtered
			"Keep-Alive":      []string{"timeout=5"},  // Should be filtered
		},
		Body:       io.NopCloser(strings.NewReader("request body")),
		RemoteAddr: "192.168.1.1:12345",
		Host:       "example.com",
		TLS:        true,
	}

	// Execute request
	resp, err := client.SendRequest(context.Background(), coreReq)

	// Verify response
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "response-value", resp.Headers.Get("X-Response-Header"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "test response body", string(body))
	resp.Body.Close()
}

func TestClient_SendRequest_NilRequest(t *testing.T) {
	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	resp, err := client.SendRequest(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestClient_SendRequest_NilURL(t *testing.T) {
	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	coreReq := &request.Request{
		Method: "GET",
		URL:    nil, // Missing URL
	}

	resp, err := client.SendRequest(context.Background(), coreReq)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "URL is nil")
}

func TestClient_SendRequest_Timeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with short timeout
	client, err := New(Config{Timeout: 50 * time.Millisecond})
	require.NoError(t, err)

	serverURL, err := url.Parse(server.URL + "/test")
	require.NoError(t, err)

	coreReq := &request.Request{
		Method:     "GET",
		URL:        serverURL,
		Headers:    http.Header{},
		Body:       http.NoBody,
		RemoteAddr: "192.168.1.1:12345",
	}

	// Execute request
	resp, err := client.SendRequest(context.Background(), coreReq)

	// Verify timeout error
	require.Error(t, err)
	assert.Nil(t, resp)

	var backendErr *core.BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.ErrorIs(t, backendErr.Err, core.ErrBackendTimeout)
	assert.Equal(t, server.URL+"/test", backendErr.BackendURL)
}

func TestClient_SendRequest_ContextCancelled(t *testing.T) {
	// Create server that delays
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	serverURL, err := url.Parse(server.URL + "/test")
	require.NoError(t, err)

	coreReq := &request.Request{
		Method:     "GET",
		URL:        serverURL,
		Headers:    http.Header{},
		Body:       http.NoBody,
		RemoteAddr: "192.168.1.1:12345",
	}

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Execute request
	resp, err := client.SendRequest(ctx, coreReq)

	// Verify cancellation error (not timeout, but failed)
	require.Error(t, err)
	assert.Nil(t, resp)

	var backendErr *core.BackendError
	require.ErrorAs(t, err, &backendErr)
	// Context cancellation should be treated as backend failed (not timeout)
	assert.ErrorIs(t, backendErr.Err, core.ErrBackendFailed)
}

func TestClient_SendRequest_InvalidURL(t *testing.T) {
	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	// Create invalid URL - this will fail during http.NewRequest
	invalidURL := &url.URL{
		Scheme: "invalid-scheme",
		Host:   "invalid host with spaces",
	}

	coreReq := &request.Request{
		Method:     "GET",
		URL:        invalidURL,
		Headers:    http.Header{},
		Body:       http.NoBody,
		RemoteAddr: "192.168.1.1:12345",
	}

	// Execute request
	resp, err := client.SendRequest(context.Background(), coreReq)

	// Verify error - request creation failure returns plain error, not BackendError
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create HTTP request")
}

func TestClient_SendRequest_XForwardedForAppends(t *testing.T) {
	// Create test server to capture headers
	var capturedXFF string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedXFF = r.Header.Get("X-Forwarded-For")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	serverURL, err := url.Parse(server.URL + "/test")
	require.NoError(t, err)

	// Request with existing X-Forwarded-For
	coreReq := &request.Request{
		Method: "GET",
		URL:    serverURL,
		Headers: http.Header{
			"X-Forwarded-For": []string{"10.0.0.1, 10.0.0.2"},
		},
		Body:       http.NoBody,
		RemoteAddr: "192.168.1.1:12345",
	}

	// Execute request
	resp, err := client.SendRequest(context.Background(), coreReq)
	require.NoError(t, err)
	require.NotNil(t, resp)
	resp.Body.Close()

	// Verify X-Forwarded-For was appended
	assert.Equal(t, "10.0.0.1, 10.0.0.2, 192.168.1.1", capturedXFF)
}

func TestClient_SendRequest_HTTPProtocol(t *testing.T) {
	// Create test server to capture headers
	var capturedProto string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedProto = r.Header.Get("X-Forwarded-Proto")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{Timeout: 5 * time.Second})
	require.NoError(t, err)

	serverURL, err := url.Parse(server.URL + "/test")
	require.NoError(t, err)

	// Request with TLS = false
	coreReq := &request.Request{
		Method:     "GET",
		URL:        serverURL,
		Headers:    http.Header{},
		Body:       http.NoBody,
		RemoteAddr: "192.168.1.1:12345",
		TLS:        false, // HTTP not HTTPS
	}

	// Execute request
	resp, err := client.SendRequest(context.Background(), coreReq)
	require.NoError(t, err)
	require.NotNil(t, resp)
	resp.Body.Close()

	// Verify X-Forwarded-Proto is http
	assert.Equal(t, "http", capturedProto)
}

func TestClient_CopyHeaders_FiltersHopByHop(t *testing.T) {
	client, err := New(Config{})
	require.NoError(t, err)

	src := http.Header{
		"Content-Type":        []string{"application/json"},
		"X-Custom-Header":     []string{"custom-value"},
		"Connection":          []string{"keep-alive"},
		"Keep-Alive":          []string{"timeout=5"},
		"Proxy-Authorization": []string{"Basic xyz"},
		"Te":                  []string{"trailers"},
		"Trailers":            []string{"X-Custom-Trailer"},
		"Transfer-Encoding":   []string{"chunked"},
		"Upgrade":             []string{"websocket"},
		"Proxy-Authenticate":  []string{"Basic"},
	}

	dst := http.Header{}
	client.copyHeaders(dst, src)

	// Verify regular headers were copied
	assert.Equal(t, "application/json", dst.Get("Content-Type"))
	assert.Equal(t, "custom-value", dst.Get("X-Custom-Header"))

	// Verify hop-by-hop headers were NOT copied
	assert.Empty(t, dst.Get("Connection"))
	assert.Empty(t, dst.Get("Keep-Alive"))
	assert.Empty(t, dst.Get("Proxy-Authorization"))
	assert.Empty(t, dst.Get("Te"))
	assert.Empty(t, dst.Get("Trailers"))
	assert.Empty(t, dst.Get("Transfer-Encoding"))
	assert.Empty(t, dst.Get("Upgrade"))
	assert.Empty(t, dst.Get("Proxy-Authenticate"))
}

func TestClient_ExtractClientIP(t *testing.T) {
	client, err := New(Config{})
	require.NoError(t, err)

	tests := []struct {
		name       string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "IP without port",
			remoteAddr: "192.168.1.1",
			expectedIP: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := client.extractClientIP(tt.remoteAddr)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestClient_IsHopByHopHeader(t *testing.T) {
	client, err := New(Config{})
	require.NoError(t, err)

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

	for _, header := range hopByHopHeaders {
		t.Run(header, func(t *testing.T) {
			assert.True(t, client.isHopByHopHeader(header))
			// Test case-insensitive
			assert.True(t, client.isHopByHopHeader(strings.ToLower(header)))
			assert.True(t, client.isHopByHopHeader(strings.ToUpper(header)))
		})
	}

	// Test non-hop-by-hop headers
	nonHopByHop := []string{
		"Content-Type",
		"X-Custom-Header",
		"Authorization",
		"Accept",
	}

	for _, header := range nonHopByHop {
		t.Run(header+" should not be hop-by-hop", func(t *testing.T) {
			assert.False(t, client.isHopByHopHeader(header))
		})
	}
}
