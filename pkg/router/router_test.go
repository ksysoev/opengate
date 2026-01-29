//go:build !compile

package router

import (
	"testing"

	"github.com/ksysoev/opengate/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_AddRoute(t *testing.T) {
	tests := []struct {
		name    string
		route   spec.Route
		wantErr bool
	}{
		{
			name: "Simple route",
			route: spec.Route{
				Path:   "/users",
				Method: "GET",
				Handler: spec.Handler{
					BaseURL: "http://backend.com",
				},
			},
			wantErr: false,
		},
		{
			name: "Route with path parameter",
			route: spec.Route{
				Path:   "/users/{id}",
				Method: "GET",
				Handler: spec.Handler{
					BaseURL: "http://backend.com",
				},
			},
			wantErr: false,
		},
		{
			name: "Route with multiple parameters",
			route: spec.Route{
				Path:   "/users/{userId}/posts/{postId}",
				Method: "GET",
				Handler: spec.Handler{
					BaseURL: "http://backend.com",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			err := r.AddRoute(tt.route)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRouter_Match(t *testing.T) {
	tests := []struct {
		wantParams  map[string]string
		name        string
		method      string
		path        string
		wantBaseURL string
		routes      []spec.Route
		wantMatch   bool
	}{
		{
			name: "Exact match",
			routes: []spec.Route{
				{Path: "/users", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend.com"}},
			},
			method:      "GET",
			path:        "/users",
			wantMatch:   true,
			wantParams:  map[string]string{},
			wantBaseURL: "http://backend.com",
		},
		{
			name: "Match with single parameter",
			routes: []spec.Route{
				{Path: "/users/{id}", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend.com"}},
			},
			method:      "GET",
			path:        "/users/123",
			wantMatch:   true,
			wantParams:  map[string]string{"id": "123"},
			wantBaseURL: "http://backend.com",
		},
		{
			name: "Match with multiple parameters",
			routes: []spec.Route{
				{Path: "/users/{userId}/posts/{postId}", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend.com"}},
			},
			method:      "GET",
			path:        "/users/123/posts/456",
			wantMatch:   true,
			wantParams:  map[string]string{"userId": "123", "postId": "456"},
			wantBaseURL: "http://backend.com",
		},
		{
			name: "No match - wrong method",
			routes: []spec.Route{
				{Path: "/users", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend.com"}},
			},
			method:    "POST",
			path:      "/users",
			wantMatch: false,
		},
		{
			name: "No match - wrong path",
			routes: []spec.Route{
				{Path: "/users", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend.com"}},
			},
			method:    "GET",
			path:      "/posts",
			wantMatch: false,
		},
		{
			name: "Match first matching route",
			routes: []spec.Route{
				{Path: "/users/{id}", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend1.com"}},
				{Path: "/users/{id}", Method: "GET", Handler: spec.Handler{BaseURL: "http://backend2.com"}},
			},
			method:      "GET",
			path:        "/users/123",
			wantMatch:   true,
			wantParams:  map[string]string{"id": "123"},
			wantBaseURL: "http://backend1.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			for _, route := range tt.routes {
				require.NoError(t, r.AddRoute(route))
			}

			route, params, err := r.Match(tt.method, tt.path)

			if !tt.wantMatch {
				assert.Error(t, err)
				assert.Nil(t, route)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, route)
			assert.Equal(t, tt.wantParams, params)
			assert.Equal(t, tt.wantBaseURL, route.Handler.BaseURL)
		})
	}
}

func TestRouter_GetRoutes(t *testing.T) {
	r := New()

	routes := []spec.Route{
		{Path: "/users", Method: "GET"},
		{Path: "/users/{id}", Method: "GET"},
		{Path: "/posts", Method: "POST"},
	}

	for _, route := range routes {
		require.NoError(t, r.AddRoute(route))
	}

	gotRoutes := r.GetRoutes()
	assert.Len(t, gotRoutes, 3)
}

func TestPathToRegex(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		testPath   string
		wantParams []string
		wantMatch  bool
	}{
		{
			name:       "Simple path",
			path:       "/users",
			testPath:   "/users",
			wantMatch:  true,
			wantParams: nil,
		},
		{
			name:       "Single parameter",
			path:       "/users/{id}",
			testPath:   "/users/123",
			wantMatch:  true,
			wantParams: []string{"id"},
		},
		{
			name:       "Multiple parameters",
			path:       "/users/{userId}/posts/{postId}",
			testPath:   "/users/123/posts/456",
			wantMatch:  true,
			wantParams: []string{"userId", "postId"},
		},
		{
			name:       "No match - extra segment",
			path:       "/users",
			testPath:   "/users/123",
			wantMatch:  false,
			wantParams: nil,
		},
		{
			name:       "No match - missing segment",
			path:       "/users/{id}",
			testPath:   "/users",
			wantMatch:  false,
			wantParams: []string{"id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, params, err := pathToRegex(tt.path)
			require.NoError(t, err)

			if tt.wantParams != nil {
				assert.Equal(t, tt.wantParams, params)
			}

			matches := pattern.MatchString(tt.testPath)
			assert.Equal(t, tt.wantMatch, matches)
		})
	}
}
