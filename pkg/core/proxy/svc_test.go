package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/ksysoev/opengate/pkg/core/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ServeHTTP_RedirectsNotFollowed(t *testing.T) {
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

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			w := httptest.NewRecorder()

			// Add route to context
			ctx := router.WithRoute(context.Background(), &route.Route{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:    "forward",
					BaseURL: backend.URL,
				},
			})
			req = req.WithContext(ctx)

			// Execute the request
			handler.ServeHTTP(w, req)

			// Verify the redirect is passed through, not followed
			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Equal(t, tt.expectedLocation, w.Header().Get("Location"))
		})
	}
}

func TestHandler_ServeHTTP_Success(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Test-Header", "test-value")

	w := httptest.NewRecorder()

	ctx := router.WithRoute(context.Background(), &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: backend.URL,
		},
	})
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, `{"status":"ok"}`, w.Body.String())
}

func TestHandler_ServeHTTP_NoRouteInContext(t *testing.T) {
	handler := New()

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ServeHTTP_NoBackendURL(t *testing.T) {
	handler := New()

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	ctx := router.WithRoute(context.Background(), &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "",
		},
	})
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ServeHTTP_InvalidBackendURL(t *testing.T) {
	handler := New()

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	ctx := router.WithRoute(context.Background(), &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "://invalid-url",
		},
	})
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandler_ServeHTTP_BackendTimeout(t *testing.T) {
	// Create a backend server that delays response
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create handler with short timeout
	handler := NewWithTimeout(50 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	ctx := router.WithRoute(context.Background(), &route.Route{
		Path:   "/test",
		Method: "GET",
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: backend.URL,
		},
	})
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandler_BuildBackendURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		requestPath string
		query       string
		wantURL     string
		wantErr     bool
	}{
		{
			name:        "Simple path",
			baseURL:     "http://backend.com",
			requestPath: "/api/users",
			query:       "",
			wantURL:     "http://backend.com/api/users",
			wantErr:     false,
		},
		{
			name:        "Base URL with path",
			baseURL:     "http://backend.com/v1",
			requestPath: "/users",
			query:       "",
			wantURL:     "http://backend.com/v1/users",
			wantErr:     false,
		},
		{
			name:        "With query parameters",
			baseURL:     "http://backend.com",
			requestPath: "/api/users",
			query:       "page=1&limit=10",
			wantURL:     "http://backend.com/api/users?page=1&limit=10",
			wantErr:     false,
		},
		{
			name:        "Invalid base URL",
			baseURL:     "://invalid",
			requestPath: "/test",
			query:       "",
			wantURL:     "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New()

			reqURL := &url.URL{
				Path:     tt.requestPath,
				RawQuery: tt.query,
			}

			gotURL, err := handler.buildBackendURL(tt.baseURL, reqURL)

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
