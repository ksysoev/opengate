package request

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest_StructFields(t *testing.T) {
	tests := []struct {
		name           string
		req            *Request
		expectedMethod string
		expectedPath   string
	}{
		{
			name: "All fields populated",
			req: &Request{
				Method: "GET",
				Path:   "/users/123",
				PathParams: map[string]string{
					"id": "123",
				},
				QueryParams: url.Values{
					"filter": []string{"active"},
				},
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:       io.NopCloser(strings.NewReader("test body")),
				RemoteAddr: "192.168.1.1:12345",
				TLS:        true,
				Host:       "example.com",
			},
			expectedMethod: "GET",
			expectedPath:   "/users/123",
		},
		{
			name: "Minimal fields",
			req: &Request{
				Method: "POST",
				Path:   "/api/v1/resource",
			},
			expectedMethod: "POST",
			expectedPath:   "/api/v1/resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.req)
			assert.Equal(t, tt.expectedMethod, tt.req.Method)
			assert.Equal(t, tt.expectedPath, tt.req.Path)
		})
	}
}

func TestResponse_StructFields(t *testing.T) {
	tests := []struct {
		resp               *Response
		name               string
		expectedStatusCode int
	}{
		{
			name: "Complete response",
			resp: &Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader("response body")),
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Minimal response",
			resp: &Response{
				StatusCode: http.StatusNoContent,
				Headers:    http.Header{},
				Body:       http.NoBody,
			},
			expectedStatusCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.resp)
			assert.Equal(t, tt.expectedStatusCode, tt.resp.StatusCode)
			assert.NotNil(t, tt.resp.Headers)
		})
	}
}
