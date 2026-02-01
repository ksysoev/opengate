package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockHTTPProvider is a mock for HTTPProvider interface
type MockHTTPProvider struct {
	mock.Mock
}

func (m *MockHTTPProvider) SendRequest(ctx context.Context, req *request.Request) (*request.Response, error) {
	args := m.Called(ctx, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	resp, ok := args.Get(0).(*request.Response)
	if !ok {
		return nil, args.Error(1)
	}

	return resp, args.Error(1)
}

func TestNewRuntime_ValidHTTPProvider(t *testing.T) {
	mockProvider := &MockHTTPProvider{}

	runtime, err := NewRuntime(mockProvider)

	assert.NoError(t, err)
	assert.NotNil(t, runtime)
}

func TestNewRuntime_NilHTTPProvider(t *testing.T) {
	runtime, err := NewRuntime(nil)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRuntime)
	assert.Nil(t, runtime)
}

func TestRuntimeImpl_SendRequest_Success(t *testing.T) {
	mockProvider := &MockHTTPProvider{}

	// Create expected request
	targetURL, err := url.Parse("https://backend.example.com/api/users?page=1")
	require.NoError(t, err)

	testReq := &request.Request{
		Method:      http.MethodGet,
		URL:         targetURL,
		Path:        "/api/users",
		QueryParams: url.Values{"page": []string{"1"}},
		Headers:     http.Header{"Authorization": []string{"Bearer token"}},
		Body:        http.NoBody,
		RemoteAddr:  "192.168.1.1:12345",
		Host:        "api.example.com",
		TLS:         true,
	}

	expectedResp := &request.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
	}

	// Setup mock expectation
	mockProvider.On("SendRequest", mock.Anything, testReq).Return(expectedResp, nil)

	// Create runtime
	runtime, err := NewRuntime(mockProvider)
	require.NoError(t, err)

	// Execute
	resp, err := runtime.SendRequest(context.Background(), testReq)

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockProvider.AssertExpectations(t)
}

func TestRuntimeImpl_SendRequest_Error(t *testing.T) {
	mockProvider := &MockHTTPProvider{}

	targetURL, err := url.Parse("https://backend.example.com/api/users")
	require.NoError(t, err)

	testReq := &request.Request{
		Method: http.MethodGet,
		URL:    targetURL,
		Body:   http.NoBody,
	}

	expectedErr := &BackendError{
		Err:        errors.New("connection refused"),
		BackendURL: "https://backend.example.com/api/users",
	}

	// Setup mock expectation
	mockProvider.On("SendRequest", mock.Anything, testReq).Return(nil, expectedErr)

	// Create runtime
	runtime, err := NewRuntime(mockProvider)
	require.NoError(t, err)

	// Execute
	resp, err := runtime.SendRequest(context.Background(), testReq)

	// Verify
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, expectedErr, err)
	mockProvider.AssertExpectations(t)
}

func TestRuntimeImpl_SendRequest_Timeout(t *testing.T) {
	mockProvider := &MockHTTPProvider{}

	targetURL, err := url.Parse("https://backend.example.com/slow")
	require.NoError(t, err)

	testReq := &request.Request{
		Method: http.MethodGet,
		URL:    targetURL,
		Body:   http.NoBody,
	}

	timeoutErr := &BackendError{
		Err:        ErrBackendTimeout,
		BackendURL: "https://backend.example.com/slow",
	}

	// Setup mock expectation
	mockProvider.On("SendRequest", mock.Anything, testReq).Return(nil, timeoutErr)

	// Create runtime
	runtime, err := NewRuntime(mockProvider)
	require.NoError(t, err)

	// Execute
	resp, err := runtime.SendRequest(context.Background(), testReq)

	// Verify
	assert.Error(t, err)
	assert.Nil(t, resp)

	var backendErr *BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.ErrorIs(t, backendErr.Err, ErrBackendTimeout)
	mockProvider.AssertExpectations(t)
}

func TestRuntimeImpl_SendRequest_ContextPropagation(t *testing.T) {
	mockProvider := &MockHTTPProvider{}

	targetURL, err := url.Parse("https://backend.example.com/test")
	require.NoError(t, err)

	testReq := &request.Request{
		Method: http.MethodGet,
		URL:    targetURL,
		Body:   http.NoBody,
	}

	expectedResp := &request.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}

	// Create context with value
	type contextKey struct{}

	ctx := context.WithValue(context.Background(), contextKey{}, "test-value")

	// Setup mock expectation with context matcher
	mockProvider.On("SendRequest", mock.MatchedBy(func(c context.Context) bool {
		// Verify context value is propagated
		return c.Value(contextKey{}) == "test-value"
	}), testReq).Return(expectedResp, nil)

	// Create runtime
	runtime, err := NewRuntime(mockProvider)
	require.NoError(t, err)

	// Execute with custom context
	resp, err := runtime.SendRequest(ctx, testReq)

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockProvider.AssertExpectations(t)
}
