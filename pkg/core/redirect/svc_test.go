package redirect

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Handle(t *testing.T) {
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
			name: "Successful 303 redirect",
			route: &route.Route{
				Path:   "/see-other",
				Method: "POST",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "https://example.com/result",
					StatusCode: http.StatusSeeOther,
				},
			},
			expectedStatus: http.StatusSeeOther,
			expectedLoc:    "https://example.com/result",
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
			wantErr: true,
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
			wantErr: true,
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
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			resp, err := handler.Handle(context.Background(), coreReq, tt.route)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)

				// Verify error is RedirectError
				var redirectErr *core.RedirectError
				assert.True(t, errors.As(err, &redirectErr))
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)
				assert.Equal(t, tt.expectedLoc, resp.Headers.Get("Location"))
				assert.Equal(t, http.NoBody, resp.Body)
			}
		})
	}
}

func TestHandler_Handle_ErrorTypes(t *testing.T) {
	handler := New()

	tests := []struct {
		route      *route.Route
		checkError func(*testing.T, error)
		name       string
	}{
		{
			name: "Missing location error",
			route: &route.Route{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "",
					StatusCode: http.StatusMovedPermanently,
				},
			},
			checkError: func(t *testing.T, err error) {
				t.Helper()

				var redirectErr *core.RedirectError
				require.True(t, errors.As(err, &redirectErr))
				assert.Contains(t, redirectErr.Reason, "no redirect location")
			},
		},
		{
			name: "Missing status code error",
			route: &route.Route{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "https://example.com",
					StatusCode: 0,
				},
			},
			checkError: func(t *testing.T, err error) {
				t.Helper()

				var redirectErr *core.RedirectError
				require.True(t, errors.As(err, &redirectErr))
				assert.Contains(t, redirectErr.Reason, "no redirect status code")
			},
		},
		{
			name: "Invalid status code error",
			route: &route.Route{
				Path:   "/test",
				Method: "GET",
				Handler: route.Handler{
					Type:       "redirect",
					Location:   "https://example.com",
					StatusCode: http.StatusOK,
				},
			},
			checkError: func(t *testing.T, err error) {
				t.Helper()

				var redirectErr *core.RedirectError
				require.True(t, errors.As(err, &redirectErr))
				assert.Contains(t, redirectErr.Reason, "invalid redirect status code")
				assert.Equal(t, http.StatusOK, redirectErr.StatusCode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			resp, err := handler.Handle(context.Background(), coreReq, tt.route)

			require.Error(t, err)
			assert.Nil(t, resp)
			tt.checkError(t, err)
		})
	}
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

func TestNew(t *testing.T) {
	handler := New()
	assert.NotNil(t, handler)
}
