package redirect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/ksysoev/opengate/pkg/core/router"
	"github.com/stretchr/testify/assert"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		route          *route.Route
		name           string
		expectedLoc    string
		expectedStatus int
		wantErr        bool
	}{
		{
			name: "Successful 301 redirect",
			route: &route.Route{
				Path:   "/old-path",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "https://example.com/new-path",
					StatusCode: http.StatusMovedPermanently,
				},
			},
			expectedStatus: http.StatusMovedPermanently,
			expectedLoc:    "https://example.com/new-path",
			wantErr:        false,
		},
		{
			name: "Successful 302 redirect",
			route: &route.Route{
				Path:   "/temp-path",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "https://example.com/temp",
					StatusCode: http.StatusFound,
				},
			},
			expectedStatus: http.StatusFound,
			expectedLoc:    "https://example.com/temp",
			wantErr:        false,
		},
		{
			name: "Successful 307 redirect",
			route: &route.Route{
				Path:   "/temp-redirect",
				Method: "POST",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "/api/v2/endpoint",
					StatusCode: http.StatusTemporaryRedirect,
				},
			},
			expectedStatus: http.StatusTemporaryRedirect,
			expectedLoc:    "/api/v2/endpoint",
			wantErr:        false,
		},
		{
			name: "Successful 308 redirect",
			route: &route.Route{
				Path:   "/permanent-redirect",
				Method: "POST",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "/api/v2/endpoint",
					StatusCode: http.StatusPermanentRedirect,
				},
			},
			expectedStatus: http.StatusPermanentRedirect,
			expectedLoc:    "/api/v2/endpoint",
			wantErr:        false,
		},
		{
			name: "Missing location",
			route: &route.Route{
				Path:   "/bad-redirect",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "",
					StatusCode: http.StatusMovedPermanently,
				},
			},
			expectedStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name: "Missing status code",
			route: &route.Route{
				Path:   "/bad-redirect",
				Method: "GET",
				Handler: route.Handler{
					Type:     "redirect",
					Location: "https://example.com",
				},
			},
			expectedStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name: "Invalid status code",
			route: &route.Route{
				Path:   "/bad-redirect",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "https://example.com",
					StatusCode: http.StatusOK,
				},
			},
			expectedStatus: http.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New()

			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			w := httptest.NewRecorder()

			ctx := context.Background()
			if tt.route != nil {
				ctx = router.WithRoute(ctx, tt.route)
			}

			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.wantErr {
				assert.Equal(t, tt.expectedLoc, w.Header().Get("Location"))
			}
		})
	}
}

func TestHandler_ServeHTTP_NoRouteInContext(t *testing.T) {
	handler := New()

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestIsValidRedirectStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{
			name:       "301 is valid",
			statusCode: http.StatusMovedPermanently,
			want:       true,
		},
		{
			name:       "302 is valid",
			statusCode: http.StatusFound,
			want:       true,
		},
		{
			name:       "303 is valid",
			statusCode: http.StatusSeeOther,
			want:       true,
		},
		{
			name:       "307 is valid",
			statusCode: http.StatusTemporaryRedirect,
			want:       true,
		},
		{
			name:       "308 is valid",
			statusCode: http.StatusPermanentRedirect,
			want:       true,
		},
		{
			name:       "200 is invalid",
			statusCode: http.StatusOK,
			want:       false,
		},
		{
			name:       "404 is invalid",
			statusCode: http.StatusNotFound,
			want:       false,
		},
		{
			name:       "500 is invalid",
			statusCode: http.StatusInternalServerError,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRedirectStatus(tt.statusCode)
			assert.Equal(t, tt.want, result)
		})
	}
}
