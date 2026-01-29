//go:build !compile

package core

import (
	"context"
	"testing"

	"github.com/ksysoev/opengate/pkg/spec"
	"github.com/stretchr/testify/assert"
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
				parser.EXPECT().ParseFile("/path/to/spec.json").Return([]spec.Route{
					{Path: "/users", Method: "GET"},
					{Path: "/users/{id}", Method: "GET"},
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
	parser.EXPECT().ParseFile("/path/to/spec.json").Return([]spec.Route{
		{Path: "/users", Method: "GET"},
		{Path: "/posts", Method: "POST"},
	}, nil)

	svc := New(parser)
	require.NoError(t, svc.LoadSpec(context.Background(), "/path/to/spec.json"))

	routes := svc.GetRoutes(context.Background())
	assert.Len(t, routes, 2)
	assert.Equal(t, "/users", routes[0].Path)
	assert.Equal(t, "GET", routes[0].Method)
}
