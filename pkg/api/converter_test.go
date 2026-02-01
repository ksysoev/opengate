package api

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPToCore(t *testing.T) {
	tests := []struct {
		setupRequest func() *http.Request
		pathParams   map[string]string
		validate     func(*testing.T, *request.Request)
		name         string
		wantErr      bool
	}{
		{
			name: "Complete request with all fields",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/api/users/123?page=1&limit=10", strings.NewReader("test body"))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer token")
				req.RemoteAddr = "192.168.1.1:12345"
				req.Host = "example.com"
				req.TLS = &tls.ConnectionState{}

				return req
			},
			pathParams: map[string]string{"id": "123"},
			validate: func(t *testing.T, coreReq *request.Request) {
				t.Helper()
				assert.Equal(t, http.MethodPost, coreReq.Method)
				assert.Equal(t, "/api/users/123", coreReq.Path)
				assert.Equal(t, map[string]string{"id": "123"}, coreReq.PathParams)
				assert.Equal(t, "1", coreReq.QueryParams.Get("page"))
				assert.Equal(t, "10", coreReq.QueryParams.Get("limit"))
				assert.Equal(t, "application/json", coreReq.Headers.Get("Content-Type"))
				assert.Equal(t, "Bearer token", coreReq.Headers.Get("Authorization"))
				assert.Equal(t, "192.168.1.1:12345", coreReq.RemoteAddr)
				assert.Equal(t, "example.com", coreReq.Host)
				assert.True(t, coreReq.TLS)

				// Read body
				body, err := io.ReadAll(coreReq.Body)
				require.NoError(t, err)
				assert.Equal(t, "test body", string(body))
			},
			wantErr: false,
		},
		{
			name: "GET request without body",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
				req.RemoteAddr = "10.0.0.1:54321"
				req.Host = "api.example.com"

				return req
			},
			pathParams: make(map[string]string),
			validate: func(t *testing.T, coreReq *request.Request) {
				t.Helper()
				assert.Equal(t, http.MethodGet, coreReq.Method)
				assert.Equal(t, "/api/users", coreReq.Path)
				assert.Equal(t, "10.0.0.1:54321", coreReq.RemoteAddr)
				assert.Equal(t, "api.example.com", coreReq.Host)
				assert.False(t, coreReq.TLS)
				assert.Equal(t, http.NoBody, coreReq.Body)
			},
			wantErr: false,
		},
		{
			name: "Request with HTTPS (TLS)",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/secure", http.NoBody)
				req.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}

				return req
			},
			pathParams: make(map[string]string),
			validate: func(t *testing.T, coreReq *request.Request) {
				t.Helper()
				assert.True(t, coreReq.TLS)
			},
			wantErr: false,
		},
		{
			name: "Request without TLS",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/insecure", http.NoBody)
				req.TLS = nil

				return req
			},
			pathParams: make(map[string]string),
			validate: func(t *testing.T, coreReq *request.Request) {
				t.Helper()
				assert.False(t, coreReq.TLS)
			},
			wantErr: false,
		},
		{
			name:         "Nil request",
			setupRequest: func() *http.Request { return nil },
			pathParams:   make(map[string]string),
			validate:     nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq := tt.setupRequest()

			coreReq, err := HTTPToCore(httpReq, tt.pathParams)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, coreReq)
			} else {
				require.NoError(t, err)
				require.NotNil(t, coreReq)

				if tt.validate != nil {
					tt.validate(t, coreReq)
				}
			}
		})
	}
}

func TestCoreToHTTP(t *testing.T) {
	tests := []struct {
		resp     *request.Response
		validate func(*testing.T, *httptest.ResponseRecorder)
		name     string
		wantErr  bool
	}{
		{
			name: "Complete response with headers and body",
			resp: &request.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type":  []string{"application/json"},
					"Cache-Control": []string{"no-cache"},
					"X-Custom":      []string{"value1", "value2"},
				},
				Body: io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
				assert.Equal(t, []string{"value1", "value2"}, w.Header()["X-Custom"])
				assert.Equal(t, `{"status":"ok"}`, w.Body.String())
			},
			wantErr: false,
		},
		{
			name: "Response with no body",
			resp: &request.Response{
				StatusCode: http.StatusNoContent,
				Headers: http.Header{
					"X-Request-ID": []string{"12345"},
				},
				Body: http.NoBody,
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				assert.Equal(t, http.StatusNoContent, w.Code)
				assert.Equal(t, "12345", w.Header().Get("X-Request-ID"))
				assert.Empty(t, w.Body.String())
			},
			wantErr: false,
		},
		{
			name: "Redirect response",
			resp: &request.Response{
				StatusCode: http.StatusMovedPermanently,
				Headers: http.Header{
					"Location": []string{"https://example.com/new-location"},
				},
				Body: http.NoBody,
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				assert.Equal(t, http.StatusMovedPermanently, w.Code)
				assert.Equal(t, "https://example.com/new-location", w.Header().Get("Location"))
			},
			wantErr: false,
		},
		{
			name: "Response with nil body",
			resp: &request.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{},
				Body:       nil,
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				assert.Equal(t, http.StatusOK, w.Code)
			},
			wantErr: false,
		},
		{
			name:     "Nil response",
			resp:     nil,
			validate: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			err := CoreToHTTP(w, tt.resp)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				if tt.validate != nil {
					tt.validate(t, w)
				}
			}
		})
	}
}

func TestHTTPToCore_QueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=golang&category=tutorial&category=example", http.NoBody)

	coreReq, err := HTTPToCore(req, map[string]string{})

	require.NoError(t, err)
	assert.Equal(t, "golang", coreReq.QueryParams.Get("q"))
	assert.Equal(t, []string{"tutorial", "example"}, coreReq.QueryParams["category"])
}

func TestHTTPToCore_PathParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", http.NoBody)

	pathParams := map[string]string{
		"userId": "123",
		"postId": "456",
	}

	coreReq, err := HTTPToCore(req, pathParams)

	require.NoError(t, err)
	assert.Equal(t, "123", coreReq.PathParams["userId"])
	assert.Equal(t, "456", coreReq.PathParams["postId"])
}

func TestHTTPToCore_EmptyPathParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)

	// Passing nil path params
	coreReq, err := HTTPToCore(req, nil)

	require.NoError(t, err)
	assert.Nil(t, coreReq.PathParams)
}

func TestCoreToHTTP_MultipleHeaderValues(t *testing.T) {
	resp := &request.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Set-Cookie": []string{"session=abc123", "token=xyz789"},
		},
		Body: http.NoBody,
	}

	w := httptest.NewRecorder()
	err := CoreToHTTP(w, resp)

	require.NoError(t, err)

	cookies := w.Header()["Set-Cookie"]
	assert.Len(t, cookies, 2)
	assert.Contains(t, cookies, "session=abc123")
	assert.Contains(t, cookies, "token=xyz789")
}

func TestHTTPToCore_PreservesHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/data", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Add("X-Custom", "value1")
	req.Header.Add("X-Custom", "value2")

	coreReq, err := HTTPToCore(req, map[string]string{})

	require.NoError(t, err)
	assert.Equal(t, "application/json", coreReq.Headers.Get("Content-Type"))
	assert.Equal(t, "Bearer token123", coreReq.Headers.Get("Authorization"))
	assert.Equal(t, []string{"value1", "value2"}, coreReq.Headers["X-Custom"])
}

func TestHTTPToCore_URLEncoding(t *testing.T) {
	encodedURL := "/api/search?q=hello%20world&filter=price%3E100"
	req := httptest.NewRequest(http.MethodGet, encodedURL, http.NoBody)

	coreReq, err := HTTPToCore(req, map[string]string{})

	require.NoError(t, err)
	// url.Values automatically decodes
	assert.Equal(t, "hello world", coreReq.QueryParams.Get("q"))
	assert.Equal(t, "price>100", coreReq.QueryParams.Get("filter"))
}

func TestRoundTrip_HTTPToCoreToHTTP(t *testing.T) {
	// Create original HTTP request
	originalReq := httptest.NewRequest(http.MethodPost, "/api/users?active=true", strings.NewReader("request data"))
	originalReq.Header.Set("Content-Type", "application/json")
	originalReq.Header.Set("Authorization", "Bearer token")

	// Convert to core
	_, err := HTTPToCore(originalReq, map[string]string{"id": "123"})
	require.NoError(t, err)

	// Create core response
	coreResp := &request.Response{
		StatusCode: http.StatusCreated,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
			"Location":     []string{"/api/users/123"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":123,"status":"created"}`)),
	}

	// Convert core response to HTTP
	w := httptest.NewRecorder()
	err = CoreToHTTP(w, coreResp)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "/api/users/123", w.Header().Get("Location"))
	assert.Equal(t, `{"id":123,"status":"created"}`, w.Body.String())
}
