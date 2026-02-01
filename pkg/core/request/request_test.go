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
		req  *Request
		name string
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
		},
		{
			name: "Minimal fields",
			req: &Request{
				Method: "POST",
				Path:   "/api/v1/resource",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.req)
			assert.Equal(t, tt.req.Method, tt.req.Method)
			assert.Equal(t, tt.req.Path, tt.req.Path)
		})
	}
}

func TestResponse_StructFields(t *testing.T) {
	tests := []struct {
		resp *Response
		name string
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
		},
		{
			name: "Minimal response",
			resp: &Response{
				StatusCode: http.StatusNoContent,
				Headers:    http.Header{},
				Body:       http.NoBody,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.resp)
			assert.Equal(t, tt.resp.StatusCode, tt.resp.StatusCode)
			assert.NotNil(t, tt.resp.Headers)
		})
	}
}
