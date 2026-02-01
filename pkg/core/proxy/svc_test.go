package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Handle_RedirectsNotFollowed(t *testing.T) {
	tests := []struct {
		name               string
		backendLocation    string
		expectedLocation   string
		backendStatusCode  int
		expectedStatusCode int
	}{
		{
			name:               "301 redirect passed through",
			backendStatusCode:  http.StatusMovedPermanently,
			backendLocation:    "https://redirect.example.com/new-location",
			expectedStatusCode: http.StatusMovedPermanently,
			expectedLocation:   "https://redirect.example.com/new-location",
		},
		{
			name:               "302 redirect passed through",
			backendStatusCode:  http.StatusFound,
			backendLocation:    "https://redirect.example.com/temp",
			expectedStatusCode: http.StatusFound,
			expectedLocation:   "https://redirect.example.com/temp",
		},
		{
			name:               "307 redirect passed through",
			backendStatusCode:  http.StatusTemporaryRedirect,
			backendLocation:    "https://redirect.example.com/temp-redirect",
			expectedStatusCode: http.StatusTemporaryRedirect,
			expectedLocation:   "https://redirect.example.com/temp-redirect",
		},
		{
			name:               "308 redirect passed through",
			backendStatusCode:  http.StatusPermanentRedirect,
			backendLocation:    "https://redirect.example.com/permanent",
			expectedStatusCode: http.StatusPermanentRedirect,
			expectedLocation:   "https://redirect.example.com/permanent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a backend server that returns a redirect
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", tt.backendLocation)
				w.WriteHeader(tt.backendStatusCode)
			}))
			defer backend.Close()

			// Create proxy handler
			handler := New()

			// Create core request
			coreReq := &request.Request{
				Method:      http.MethodGet,
				Path:        "/test",
				PathParams:  make(map[string]string),
				QueryParams: url.Values{},
				Headers:     http.Header{},
				Body:        http.NoBody,
				RemoteAddr:  "192.168.1.1:12345",
				TLS:         false,
				Host:        "example.com",
			}

			rt := &route.Route{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:    "forward",
					BaseURL: backend.URL,
				},
			}

			// Execute the request
			resp, err := handler.Handle(context.Background(), coreReq, rt)

			// Verify the redirect is passed through, not followed
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)
			assert.Equal(t, tt.expectedLocation, resp.Headers.Get("Location"))
		})
	}
}

func TestHandler_Handle_Success(t *testing.T) {
	// Create a backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are forwarded
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		assert.NotEmpty(t, r.Header.Get("X-Forwarded-For"))
		assert.NotEmpty(t, r.Header.Get("X-Forwarded-Proto"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	handler := New()

	coreReq := &request.Request{
		Method:     http.MethodGet,
		Path:       "/test",
		PathParams: make(map[string]string),
		QueryParams: url.Values{
			"page": []string{"1"},
		},
		Headers: http.Header{
			"X-Test-Header": []string{"test-value"},
		},
		Body:       http.NoBody,
		RemoteAddr: "192.168.1.1:12345",
		TLS:        false,
		Host:       "example.com",
	}

	rt := &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: backend.URL,
		},
	}

	resp, err := handler.Handle(context.Background(), coreReq, rt)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers.Get("Content-Type"))

	// Read body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"status":"ok"}`, string(body))
}

func TestHandler_Handle_NoBackendURL(t *testing.T) {
	handler := New()

	coreReq := &request.Request{
		Method:      http.MethodGet,
		Path:        "/test",
		PathParams:  make(map[string]string),
		QueryParams: url.Values{},
		Headers:     http.Header{},
		Body:        http.NoBody,
		RemoteAddr:  "192.168.1.1:12345",
		TLS:         false,
		Host:        "example.com",
	}

	rt := &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "",
		},
	}

	resp, err := handler.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, core.ErrInvalidRoute))
}

func TestHandler_Handle_InvalidBackendURL(t *testing.T) {
	handler := New()

	coreReq := &request.Request{
		Method:      http.MethodGet,
		Path:        "/test",
		PathParams:  make(map[string]string),
		QueryParams: url.Values{},
		Headers:     http.Header{},
		Body:        http.NoBody,
		RemoteAddr:  "192.168.1.1:12345",
		TLS:         false,
		Host:        "example.com",
	}

	rt := &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "://invalid-url",
		},
	}

	resp, err := handler.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestHandler_Handle_BackendTimeout(t *testing.T) {
	// Create a backend server that delays response
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create handler with short timeout
	handler := NewWithTimeout(50 * time.Millisecond)

	coreReq := &request.Request{
		Method:      http.MethodGet,
		Path:        "/test",
		PathParams:  make(map[string]string),
		QueryParams: url.Values{},
		Headers:     http.Header{},
		Body:        http.NoBody,
		RemoteAddr:  "192.168.1.1:12345",
		TLS:         false,
		Host:        "example.com",
	}

	rt := &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: backend.URL,
		},
	}

	resp, err := handler.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)

	// Verify error is BackendError
	var backendErr *core.BackendError
	assert.True(t, errors.As(err, &backendErr))
	assert.True(t, errors.Is(err, core.ErrBackendFailed))
}

func TestHandler_Handle_XForwardedHeaders(t *testing.T) {
	// Create a backend server that captures headers
	var capturedHeaders http.Header

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := New()

	tests := []struct {
		name           string
		existingXFF    string
		expectedProto  string
		expectedHost   string
		expectedXFFEnd string
		tls            bool
	}{
		{
			name:           "HTTPS request with TLS",
			tls:            true,
			existingXFF:    "",
			expectedProto:  "https",
			expectedHost:   "example.com",
			expectedXFFEnd: "192.168.1.1",
		},
		{
			name:           "HTTP request without TLS",
			tls:            false,
			existingXFF:    "",
			expectedProto:  "http",
			expectedHost:   "example.com",
			expectedXFFEnd: "192.168.1.1",
		},
		{
			name:           "Existing X-Forwarded-For",
			tls:            false,
			existingXFF:    "10.0.0.1, 10.0.0.2",
			expectedProto:  "http",
			expectedHost:   "example.com",
			expectedXFFEnd: "10.0.0.1, 10.0.0.2, 192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.existingXFF != "" {
				headers.Set("X-Forwarded-For", tt.existingXFF)
			}

			coreReq := &request.Request{
				Method:      http.MethodGet,
				Path:        "/test",
				PathParams:  make(map[string]string),
				QueryParams: url.Values{},
				Headers:     headers,
				Body:        http.NoBody,
				RemoteAddr:  "192.168.1.1:12345",
				TLS:         tt.tls,
				Host:        tt.expectedHost,
			}

			rt := &route.Route{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:    "forward",
					BaseURL: backend.URL,
				},
			}

			_, err := handler.Handle(context.Background(), coreReq, rt)
			require.NoError(t, err)

			// Verify X-Forwarded headers
			assert.Equal(t, tt.expectedProto, capturedHeaders.Get("X-Forwarded-Proto"))
			assert.Equal(t, tt.expectedHost, capturedHeaders.Get("X-Forwarded-Host"))
			assert.Equal(t, tt.expectedXFFEnd, capturedHeaders.Get("X-Forwarded-For"))
		})
	}
}

func TestHandler_BuildBackendURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		requestPath string
		query       url.Values
		wantURL     string
		wantErr     bool
	}{
		{
			name:        "Simple path",
			baseURL:     "http://backend.com",
			requestPath: "/api/users",
			query:       url.Values{},
			wantURL:     "http://backend.com/api/users",
			wantErr:     false,
		},
		{
			name:        "Base URL with path",
			baseURL:     "http://backend.com/v1",
			requestPath: "/users",
			query:       url.Values{},
			wantURL:     "http://backend.com/v1/users",
			wantErr:     false,
		},
		{
			name:        "With query parameters",
			baseURL:     "http://backend.com",
			requestPath: "/api/users",
			query: url.Values{
				"page":  []string{"1"},
				"limit": []string{"10"},
			},
			wantURL: "http://backend.com/api/users?limit=10&page=1",
			wantErr: false,
		},
		{
			name:        "Invalid base URL",
			baseURL:     "://invalid",
			requestPath: "/test",
			query:       url.Values{},
			wantURL:     "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New()

			gotURL, err := handler.buildBackendURL(tt.baseURL, tt.requestPath, tt.query)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantURL, gotURL)
			}
		})
	}
}

func TestHandler_ExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[::1]:12345",
			want:       "[::1]",
		},
		{
			name:       "Just IP without port",
			remoteAddr: "192.168.1.1",
			want:       "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New()
			got := handler.extractClientIP(tt.remoteAddr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_IsHopByHopHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"Connection is hop-by-hop", "Connection", true},
		{"Keep-Alive is hop-by-hop", "Keep-Alive", true},
		{"Transfer-Encoding is hop-by-hop", "Transfer-Encoding", true},
		{"Content-Type is not hop-by-hop", "Content-Type", false},
		{"Authorization is not hop-by-hop", "Authorization", false},
		{"Case insensitive - connection", "connection", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New()
			got := handler.isHopByHopHeader(tt.header)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_CopyHeaders(t *testing.T) {
	handler := New()

	src := http.Header{
		"Content-Type":      []string{"application/json"},
		"Authorization":     []string{"Bearer token"},
		"Connection":        []string{"keep-alive"},
		"Transfer-Encoding": []string{"chunked"},
		"X-Custom-Header":   []string{"value1", "value2"},
	}

	dst := http.Header{}
	handler.copyHeaders(dst, src)

	// Verify normal headers are copied
	assert.Equal(t, "application/json", dst.Get("Content-Type"))
	assert.Equal(t, "Bearer token", dst.Get("Authorization"))
	assert.Equal(t, []string{"value1", "value2"}, dst["X-Custom-Header"])

	// Verify hop-by-hop headers are NOT copied
	assert.Empty(t, dst.Get("Connection"))
	assert.Empty(t, dst.Get("Transfer-Encoding"))
}

func TestHandler_Handle_WithRequestBody(t *testing.T) {
	requestBody := `{"data":"test"}`

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, requestBody, string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := New()

	coreReq := &request.Request{
		Method:      http.MethodPost,
		Path:        "/test",
		PathParams:  make(map[string]string),
		QueryParams: url.Values{},
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:       io.NopCloser(strings.NewReader(requestBody)),
		RemoteAddr: "192.168.1.1:12345",
		TLS:        false,
		Host:       "example.com",
	}

	rt := &route.Route{
		Path:   "/test",
		Method: "POST",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: backend.URL,
		},
	}

	resp, err := handler.Handle(context.Background(), coreReq, rt)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNew(t *testing.T) {
	handler := New()

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.client)
	assert.Equal(t, defaultTimeout, handler.timeout)
	assert.Equal(t, defaultTimeout, handler.client.Timeout)
}

func TestNewWithTimeout(t *testing.T) {
	customTimeout := 5 * time.Second
	handler := NewWithTimeout(customTimeout)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.client)
	assert.Equal(t, customTimeout, handler.timeout)
	assert.Equal(t, customTimeout, handler.client.Timeout)
}
