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

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// createMockRuntime is a helper to create a mock runtime with HTTP client
func createMockRuntime(t *testing.T, mockHTTP *core.MockHTTPClient) core.Runtime {
	t.Helper()

	mockRuntime := core.NewMockRuntime(t)
	mockRuntime.EXPECT().GetHTTPClient().Return(mockHTTP).Maybe()

	return mockRuntime
}

func TestForwarder_Handle_RedirectsNotFollowed(t *testing.T) {
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

			// Create mock runtime
			mockHTTP := core.NewMockHTTPClient(t)
			mockRuntime := createMockRuntime(t, mockHTTP)

			// Setup expectation
			mockHTTP.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
				return req.URL.Path == "/test"
			})).Return(&http.Response{
				StatusCode: tt.backendStatusCode,
				Header: http.Header{
					"Location": []string{tt.backendLocation},
				},
				Body: http.NoBody,
			}, nil)

			// Create proxy forwarder
			forwarder, err := New(mockRuntime)
			require.NoError(t, err)

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
			resp, err := forwarder.Handle(context.Background(), coreReq, rt)

			// Verify the redirect is passed through, not followed
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)
			assert.Equal(t, tt.expectedLocation, resp.Headers.Get("Location"))
		})
	}
}

func TestForwarder_Handle_Success(t *testing.T) {
	// Create mock HTTP client
	mockHTTP := core.NewMockHTTPClient(t)
	mockRuntime := createMockRuntime(t, mockHTTP)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

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
			BaseURL: "http://backend.com",
		},
	}

	// Setup mock expectation
	mockHTTP.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		// Verify request properties
		assert.Equal(t, "/test?page=1", req.URL.Path+"?"+req.URL.RawQuery)
		assert.Equal(t, "test-value", req.Header.Get("X-Test-Header"))
		assert.NotEmpty(t, req.Header.Get("X-Forwarded-For"))
		assert.NotEmpty(t, req.Header.Get("X-Forwarded-Proto"))

		return true
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
	}, nil)

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers.Get("Content-Type"))

	// Read body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"status":"ok"}`, string(body))
}

func TestForwarder_Handle_NoBackendURL(t *testing.T) {
	mockHTTP := core.NewMockHTTPClient(t)
	mockRuntime := createMockRuntime(t, mockHTTP)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

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

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestForwarder_Handle_BackendTimeout(t *testing.T) {
	mockHTTP := core.NewMockHTTPClient(t)
	mockRuntime := createMockRuntime(t, mockHTTP)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

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
			BaseURL: "http://backend.com",
		},
	}

	// Simulate timeout error
	mockHTTP.EXPECT().Do(mock.Anything).Return(nil, context.DeadlineExceeded)

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)

	// Verify error is BackendError with timeout
	var backendErr *core.BackendError
	assert.True(t, errors.As(err, &backendErr))
	assert.True(t, errors.Is(err, core.ErrBackendTimeout))
}

func TestForwarder_Handle_XForwardedHeaders(t *testing.T) {
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
			mockHTTP := core.NewMockHTTPClient(t)
			mockRuntime := createMockRuntime(t, mockHTTP)

			forwarder, err := New(mockRuntime)
			require.NoError(t, err)

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
					BaseURL: "http://backend.com",
				},
			}

			// Setup mock to verify headers
			mockHTTP.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
				assert.Equal(t, tt.expectedProto, req.Header.Get("X-Forwarded-Proto"))
				assert.Equal(t, tt.expectedHost, req.Header.Get("X-Forwarded-Host"))
				assert.Equal(t, tt.expectedXFFEnd, req.Header.Get("X-Forwarded-For"))

				return true
			})).Return(&http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
			}, nil)

			_, err = forwarder.Handle(context.Background(), coreReq, rt)
			require.NoError(t, err)
		})
	}
}

func TestForwarder_BuildBackendURL(t *testing.T) {
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
			mockHTTP := core.NewMockHTTPClient(t)
			mockRuntime := createMockRuntime(t, mockHTTP)

			forwarder, err := New(mockRuntime)
			require.NoError(t, err)

			gotURL, err := forwarder.buildBackendURL(tt.baseURL, tt.requestPath, tt.query)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantURL, gotURL)
			}
		})
	}
}

func TestForwarder_ExtractClientIP(t *testing.T) {
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
			mockHTTP := core.NewMockHTTPClient(t)
			mockRuntime := createMockRuntime(t, mockHTTP)

			forwarder, err := New(mockRuntime)
			require.NoError(t, err)

			got := forwarder.extractClientIP(tt.remoteAddr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestForwarder_IsHopByHopHeader(t *testing.T) {
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
			mockHTTP := core.NewMockHTTPClient(t)
			mockRuntime := createMockRuntime(t, mockHTTP)

			forwarder, err := New(mockRuntime)
			require.NoError(t, err)

			got := forwarder.isHopByHopHeader(tt.header)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestForwarder_CopyHeaders(t *testing.T) {
	mockHTTP := core.NewMockHTTPClient(t)
	mockRuntime := createMockRuntime(t, mockHTTP)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	src := http.Header{
		"Content-Type":      []string{"application/json"},
		"Authorization":     []string{"Bearer token"},
		"Connection":        []string{"keep-alive"},
		"Transfer-Encoding": []string{"chunked"},
		"X-Custom-Header":   []string{"value1", "value2"},
	}

	dst := http.Header{}
	forwarder.copyHeaders(dst, src)

	// Verify normal headers are copied
	assert.Equal(t, "application/json", dst.Get("Content-Type"))
	assert.Equal(t, "Bearer token", dst.Get("Authorization"))
	assert.Equal(t, []string{"value1", "value2"}, dst["X-Custom-Header"])

	// Verify hop-by-hop headers are NOT copied
	assert.Empty(t, dst.Get("Connection"))
	assert.Empty(t, dst.Get("Transfer-Encoding"))
}

func TestForwarder_Handle_WithRequestBody(t *testing.T) {
	requestBody := `{"data":"test"}`

	mockHTTP := core.NewMockHTTPClient(t)
	mockRuntime := createMockRuntime(t, mockHTTP)

	handler, err := New(mockRuntime)
	require.NoError(t, err)

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
			BaseURL: "http://backend.com",
		},
	}

	// Setup mock expectation - verify the request has a body reader
	mockHTTP.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		// Just verify the request has a body - don't consume it
		assert.NotNil(t, req.Body)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

		return true
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       http.NoBody,
	}, nil)

	resp, err := handler.Handle(context.Background(), coreReq, rt)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNew(t *testing.T) {
	mockHTTP := core.NewMockHTTPClient(t)
	mockRuntime := createMockRuntime(t, mockHTTP)

	forwarder, err := New(mockRuntime)

	assert.NoError(t, err)
	assert.NotNil(t, forwarder)
	assert.NotNil(t, forwarder.runtime)
}

func TestNew_NilRuntime(t *testing.T) {
	forwarder, err := New(nil)

	assert.Error(t, err)
	assert.Nil(t, forwarder)
	assert.ErrorIs(t, err, core.ErrInvalidRuntime)
}

func TestNew_NilHTTPClient(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)
	mockRuntime.EXPECT().GetHTTPClient().Return(nil)

	forwarder, err := New(mockRuntime)

	assert.Error(t, err)
	assert.Nil(t, forwarder)
	assert.ErrorIs(t, err, core.ErrInvalidRuntime)
}
