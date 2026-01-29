//go:build !compile

package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	tests := []struct {
		name      string
		specJSON  string
		wantCount int
		wantErr   bool
	}{
		{
			name: "Valid spec with multiple routes",
			specJSON: `{
				"openapi": "3.1.0",
				"info": {"version": "1.0.0", "title": "Test API"},
				"paths": {
					"/users": {
						"get": {
							"operationId": "get-users",
							"x-opengate": {
								"handler": {
									"options": {"baseUrl": "http://backend.com"}
								}
							}
						},
						"post": {
							"operationId": "create-user",
							"x-opengate": {
								"handler": {
									"options": {"baseUrl": "http://backend.com"}
								}
							}
						}
					},
					"/users/{id}": {
						"get": {
							"operationId": "get-user",
							"x-opengate": {
								"handler": {
									"options": {"baseUrl": "http://backend.com"}
								}
							}
						}
					}
				}
			}`,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "Invalid JSON",
			specJSON:  `{invalid json}`,
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "Empty paths",
			specJSON: `{
				"openapi": "3.1.0",
				"info": {"version": "1.0.0", "title": "Test API"},
				"paths": {}
			}`,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			routes, err := parser.Parse([]byte(tt.specJSON))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, routes, tt.wantCount)
		})
	}
}

func TestParser_ParseFile(t *testing.T) {
	t.Run("File does not exist", func(t *testing.T) {
		parser := NewParser()
		_, err := parser.ParseFile("/non/existent/file.json")
		assert.Error(t, err)
	})
}

func TestParser_CreateRoute(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		method          string
		operation       *Operation
		wantBaseURL     string
		wantOperationID string
	}{
		{
			name:   "Route with x-opengate",
			path:   "/users",
			method: "GET",
			operation: &Operation{
				OperationID: "get-users",
				XOpenGate: &OpenGateExt{
					Handler: HandlerConfig{
						Options: HandlerOptions{
							BaseURL: "http://backend.com",
						},
					},
				},
			},
			wantBaseURL:     "http://backend.com",
			wantOperationID: "get-users",
		},
		{
			name:   "Route without extensions",
			path:   "/health",
			method: "GET",
			operation: &Operation{
				OperationID: "health-check",
			},
			wantBaseURL:     "",
			wantOperationID: "health-check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			route := parser.createRoute(tt.path, tt.method, tt.operation)

			assert.Equal(t, tt.path, route.Path)
			assert.Equal(t, tt.method, route.Method)
			assert.Equal(t, tt.wantOperationID, route.OperationID)
			assert.Equal(t, tt.wantBaseURL, route.Handler.BaseURL)
		})
	}
}
