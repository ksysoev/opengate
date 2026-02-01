package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
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

func TestNew_NilRuntime(t *testing.T) {
	forwarder, err := New(nil)

	assert.Error(t, err)
	assert.Nil(t, forwarder)
	assert.ErrorIs(t, err, core.ErrInvalidRuntime)
}

func TestNew_ValidRuntime(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	forwarder, err := New(mockRuntime)

	assert.NoError(t, err)
	assert.NotNil(t, forwarder)
}

func TestForwarder_Handle_Success(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	// Setup expectation for SendRequest
	expectedResponse := &request.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
	}

	mockRuntime.EXPECT().SendRequest(
		mock.Anything,
		mock.MatchedBy(func(req *request.Request) bool {
			// Verify URL was constructed correctly
			return req.URL != nil &&
				req.URL.Scheme == "https" &&
				req.URL.Host == "backend.example.com" &&
				req.URL.Path == "/api/users/123" &&
				req.URL.Query().Get("page") == "1" &&
				req.Method == http.MethodGet
		}),
	).Return(expectedResponse, nil)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	// Create request
	coreReq := &request.Request{
		Method:      http.MethodGet,
		Path:        "/api/users/123",
		QueryParams: url.Values{"page": []string{"1"}},
		Headers:     http.Header{},
		Body:        http.NoBody,
	}

	// Create route
	rt := &route.Route{
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "https://backend.example.com",
		},
	}

	// Execute
	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers.Get("Content-Type"))
}

func TestForwarder_Handle_EmptyBaseURL(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	coreReq := &request.Request{
		Method: http.MethodGet,
		Path:   "/test",
	}

	rt := &route.Route{
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "", // Empty base URL
		},
	}

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, core.ErrInvalidRoute)
}

func TestForwarder_Handle_InvalidBaseURL(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	coreReq := &request.Request{
		Method: http.MethodGet,
		Path:   "/test",
	}

	rt := &route.Route{
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "://invalid-url", // Invalid URL
		},
	}

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to build backend URL")
}

func TestForwarder_Handle_BackendError(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	// Setup expectation for SendRequest to return error
	backendErr := &core.BackendError{
		Err:        errors.New("connection refused"),
		BackendURL: "https://backend.example.com/test",
	}

	mockRuntime.EXPECT().SendRequest(mock.Anything, mock.Anything).Return(nil, backendErr)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	coreReq := &request.Request{
		Method: http.MethodGet,
		Path:   "/test",
	}

	rt := &route.Route{
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "https://backend.example.com",
		},
	}

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)

	var backendError *core.BackendError
	assert.ErrorAs(t, err, &backendError)
}

func TestForwarder_Handle_BackendTimeout(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	// Setup expectation for SendRequest to return timeout error
	timeoutErr := &core.BackendError{
		Err:        core.ErrBackendTimeout,
		BackendURL: "https://backend.example.com/slow",
	}

	mockRuntime.EXPECT().SendRequest(mock.Anything, mock.Anything).Return(nil, timeoutErr)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	coreReq := &request.Request{
		Method: http.MethodGet,
		Path:   "/slow",
	}

	rt := &route.Route{
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "https://backend.example.com",
		},
	}

	resp, err := forwarder.Handle(context.Background(), coreReq, rt)

	assert.Error(t, err)
	assert.Nil(t, resp)

	var backendError *core.BackendError
	require.ErrorAs(t, err, &backendError)
	assert.ErrorIs(t, backendError.Err, core.ErrBackendTimeout)
}

func TestForwarder_BuildBackendURL(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)
	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	tests := []struct {
		name        string
		baseURL     string
		reqPath     string
		queryParams url.Values
		expectedURL string
		wantErr     bool
	}{
		{
			name:        "Simple path",
			baseURL:     "https://backend.example.com",
			reqPath:     "/api/users",
			queryParams: url.Values{},
			expectedURL: "https://backend.example.com/api/users",
			wantErr:     false,
		},
		{
			name:        "Base URL with path",
			baseURL:     "https://backend.example.com/v1",
			reqPath:     "/users",
			queryParams: url.Values{},
			expectedURL: "https://backend.example.com/v1/users",
			wantErr:     false,
		},
		{
			name:        "Base URL with trailing slash",
			baseURL:     "https://backend.example.com/",
			reqPath:     "/api/users",
			queryParams: url.Values{},
			expectedURL: "https://backend.example.com/api/users",
			wantErr:     false,
		},
		{
			name:        "With query parameters",
			baseURL:     "https://backend.example.com",
			reqPath:     "/api/users",
			queryParams: url.Values{"page": []string{"1"}, "limit": []string{"10"}},
			expectedURL: "https://backend.example.com/api/users?limit=10&page=1",
			wantErr:     false,
		},
		{
			name:        "Base URL with path and query",
			baseURL:     "https://backend.example.com/v1",
			reqPath:     "/users/123",
			queryParams: url.Values{"details": []string{"true"}},
			expectedURL: "https://backend.example.com/v1/users/123?details=true",
			wantErr:     false,
		},
		{
			name:        "Invalid base URL",
			baseURL:     "://invalid",
			reqPath:     "/test",
			queryParams: url.Values{},
			expectedURL: "",
			wantErr:     true,
		},
		{
			name:        "HTTP protocol",
			baseURL:     "http://backend.example.com",
			reqPath:     "/api",
			queryParams: url.Values{},
			expectedURL: "http://backend.example.com/api",
			wantErr:     false,
		},
		{
			name:        "Port in URL",
			baseURL:     "https://backend.example.com:8080",
			reqPath:     "/api",
			queryParams: url.Values{},
			expectedURL: "https://backend.example.com:8080/api",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultURL, err := forwarder.buildBackendURL(tt.baseURL, tt.reqPath, tt.queryParams)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resultURL)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resultURL)
				assert.Equal(t, tt.expectedURL, resultURL.String())
			}
		})
	}
}

func TestForwarder_Handle_URLSetCorrectly(t *testing.T) {
	mockRuntime := core.NewMockRuntime(t)

	var capturedRequest *request.Request

	// Capture the request passed to SendRequest
	mockRuntime.EXPECT().SendRequest(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, req *request.Request) {
			capturedRequest = req
		}).
		Return(&request.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil)

	forwarder, err := New(mockRuntime)
	require.NoError(t, err)

	coreReq := &request.Request{
		Method:      http.MethodPost,
		Path:        "/api/users",
		QueryParams: url.Values{"action": []string{"create"}},
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		Body:        io.NopCloser(strings.NewReader(`{"name":"test"}`)),
	}

	rt := &route.Route{
		Handler: route.Handler{
			Type:    "forward",
			BaseURL: "https://backend.example.com/v1",
		},
	}

	_, err = forwarder.Handle(context.Background(), coreReq, rt)
	require.NoError(t, err)

	// Verify the URL was set correctly in the request
	assert.NotNil(t, capturedRequest)
	assert.NotNil(t, capturedRequest.URL)
	assert.Equal(t, "https", capturedRequest.URL.Scheme)
	assert.Equal(t, "backend.example.com", capturedRequest.URL.Host)
	assert.Equal(t, "/v1/api/users", capturedRequest.URL.Path)
	assert.Equal(t, "action=create", capturedRequest.URL.RawQuery)

	// Verify original request fields are preserved
	assert.Equal(t, http.MethodPost, capturedRequest.Method)
	assert.Equal(t, "/api/users", capturedRequest.Path) // Original path unchanged
	assert.Equal(t, "application/json", capturedRequest.Headers.Get("Content-Type"))
}
