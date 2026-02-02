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
									"type": "forward",
									"options": {"url": "http://backend.com"}
								}
							}
						},
						"post": {
							"operationId": "create-user",
							"x-opengate": {
								"handler": {
									"type": "forward",
									"options": {"url": "http://backend.com"}
								}
							}
						}
					},
					"/users/{id}": {
						"get": {
							"operationId": "get-user",
							"x-opengate": {
								"handler": {
									"type": "forward",
									"options": {"url": "http://backend.com"}
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
			name: "Valid spec with redirect routes",
			specJSON: `{
				"openapi": "3.1.0",
				"info": {"version": "1.0.0", "title": "Test API"},
				"paths": {
					"/old-path": {
						"get": {
							"operationId": "redirect-old-path",
							"x-opengate": {
								"handler": {
									"type": "redirect",
									"options": {
										"location": "https://example.com/new-path",
										"status_code": 301
									}
								}
							}
						}
					}
				}
			}`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "Invalid handler type",
			specJSON: `{
				"openapi": "3.1.0",
				"info": {"version": "1.0.0", "title": "Test API"},
				"paths": {
					"/users": {
						"get": {
							"operationId": "get-users",
							"x-opengate": {
								"handler": {
									"type": "invalid-type",
									"options": {"url": "http://backend.com"}
								}
							}
						}
					}
				}
			}`,
			wantCount: 0,
			wantErr:   true,
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
		operation       *Operation
		name            string
		path            string
		method          string
		wantType        string
		wantBaseURL     string
		wantLocation    string
		wantOperationID string
		wantErrContains string
		wantStatusCode  int
		wantErr         bool
	}{
		{
			name:   "Route with forward handler",
			path:   "/users",
			method: "GET",
			operation: &Operation{
				OperationID: "get-users",
				XOpenGate: &OpenGateExt{
					Handler: Handler{
						Type: "forward",
						Options: HandlerOptions{
							URL: "http://backend.com",
						},
					},
				},
			},
			wantType:        "forward",
			wantBaseURL:     "http://backend.com",
			wantOperationID: "get-users",
			wantErr:         false,
		},
		{
			name:   "Route with redirect handler",
			path:   "/old-path",
			method: "GET",
			operation: &Operation{
				OperationID: "redirect-old",
				XOpenGate: &OpenGateExt{
					Handler: Handler{
						Type: "redirect",
						Options: HandlerOptions{
							Location:   "https://example.com/new-path",
							StatusCode: 301,
						},
					},
				},
			},
			wantType:        "redirect",
			wantLocation:    "https://example.com/new-path",
			wantStatusCode:  301,
			wantOperationID: "redirect-old",
			wantErr:         false,
		},
		{
			name:   "Route without extensions",
			path:   "/health",
			method: "GET",
			operation: &Operation{
				OperationID: "health-check",
			},
			wantType:        "",
			wantBaseURL:     "",
			wantOperationID: "health-check",
			wantErr:         false,
		},
		{
			name:   "Route with unknown handler type",
			path:   "/test",
			method: "POST",
			operation: &Operation{
				OperationID: "test-op",
				XOpenGate: &OpenGateExt{
					Handler: Handler{
						Type: "unknown-type",
						Options: HandlerOptions{
							URL: "http://backend.com",
						},
					},
				},
			},
			wantErr:         true,
			wantErrContains: "unknown handler type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			route, err := parser.createRoute(tt.path, tt.method, tt.operation)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.path, route.Path)
			assert.Equal(t, tt.method, route.Method)
			assert.Equal(t, tt.wantOperationID, route.OperationID)
			assert.Equal(t, tt.wantType, route.Handler.Type)
			assert.Equal(t, tt.wantBaseURL, route.Handler.BaseURL)
			assert.Equal(t, tt.wantLocation, route.Handler.Location)
			assert.Equal(t, tt.wantStatusCode, route.Handler.StatusCode)
		})
	}
}
