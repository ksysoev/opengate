//go:build !compile

package core

import (
	"context"
	"net/http"
	"testing"

	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_LoadSpec(t *testing.T) {
	tests := []struct {
		setupMock func(t *testing.T, parser *MockspecParser)
		name      string
		specPath  string
		wantErr   bool
	}{
		{
			name:     "Success",
			specPath: "/path/to/spec.json",
			setupMock: func(t *testing.T, parser *MockspecParser) {
				t.Helper()
				parser.EXPECT().ParseFile("/path/to/spec.json").Return([]route.Route{
					{Path: "/users", Method: "GET", Handler: route.Handler{Type: "forward"}},
					{Path: "/users/{id}", Method: "GET", Handler: route.Handler{Type: "forward"}},
				}, nil)
			},
			wantErr: false,
		},
		{
			name:     "Parse error",
			specPath: "/path/to/invalid.json",
			setupMock: func(t *testing.T, parser *MockspecParser) {
				t.Helper()
				parser.EXPECT().ParseFile("/path/to/invalid.json").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		{
			name:     "Empty routes",
			specPath: "/path/to/empty.json",
			setupMock: func(t *testing.T, parser *MockspecParser) {
				t.Helper()
				parser.EXPECT().ParseFile("/path/to/empty.json").Return([]route.Route{}, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewMockspecParser(t)
			tt.setupMock(t, parser)

			svc := New(parser)
			err := svc.LoadSpec(context.Background(), tt.specPath)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_GetRoutes(t *testing.T) {
	parser := NewMockspecParser(t)
	parser.EXPECT().ParseFile("/path/to/spec.json").Return([]route.Route{
		{Path: "/users", Method: "GET", Handler: route.Handler{Type: "forward"}},
		{Path: "/posts", Method: "POST", Handler: route.Handler{Type: "redirect"}},
	}, nil)

	svc := New(parser)
	require.NoError(t, svc.LoadSpec(context.Background(), "/path/to/spec.json"))

	routes := svc.GetRoutes(context.Background())
	assert.Len(t, routes, 2)
	assert.Equal(t, "/users", routes[0].Path)
	assert.Equal(t, "GET", routes[0].Method)
}

func TestService_HandleRequest(t *testing.T) {
	tests := []struct {
		req         *request.Request
		setupMock   func(t *testing.T, parser *MockspecParser, handler *MockHandler)
		name        string
		errContains string
		wantErr     bool
	}{
		{
			name: "Success - forward handler",
			req: &request.Request{
				Method: "GET",
				Path:   "/users",
				Body:   http.NoBody,
			},
			setupMock: func(t *testing.T, parser *MockspecParser, handler *MockHandler) {
				t.Helper()
				parser.EXPECT().ParseFile("/path/to/spec.json").Return([]route.Route{
					{Path: "/users", Method: "GET", Handler: route.Handler{Type: "forward", BaseURL: "http://backend"}},
				}, nil)
				handler.EXPECT().Handle(mock.Anything, mock.Anything, mock.Anything).Return(&request.Response{
					StatusCode: 200,
					Headers:    http.Header{},
					Body:       http.NoBody,
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "Route not found",
			req: &request.Request{
				Method: "GET",
				Path:   "/notfound",
				Body:   http.NoBody,
			},
			setupMock: func(t *testing.T, parser *MockspecParser, handler *MockHandler) {
				t.Helper()
				parser.EXPECT().ParseFile("/path/to/spec.json").Return([]route.Route{
					{Path: "/users", Method: "GET", Handler: route.Handler{Type: "forward"}},
				}, nil)
			},
			wantErr:     true,
			errContains: "route not found",
		},
		{
			name: "Handler not registered",
			req: &request.Request{
				Method: "GET",
				Path:   "/users",
				Body:   http.NoBody,
			},
			setupMock: func(t *testing.T, parser *MockspecParser, handler *MockHandler) {
				t.Helper()
				parser.EXPECT().ParseFile("/path/to/spec.json").Return([]route.Route{
					{Path: "/users", Method: "GET", Handler: route.Handler{Type: "unknown"}},
				}, nil)
			},
			wantErr:     true,
			errContains: "no handler registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewMockspecParser(t)
			handler := NewMockHandler(t)
			tt.setupMock(t, parser, handler)

			svc := New(parser)
			svc.RegisterHandler("forward", handler)
			require.NoError(t, svc.LoadSpec(context.Background(), "/path/to/spec.json"))

			resp, err := svc.HandleRequest(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}
