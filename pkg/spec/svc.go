// Package spec provides functionality for parsing and managing OpenAPI specifications.
package spec

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ksysoev/opengate/pkg/core/route"
)

// OpenAPISpec represents the OpenAPI specification structure.
type OpenAPISpec struct {
	Paths   map[string]PathItem `json:"paths"`
	Info    Info                `json:"info"`
	OpenAPI string              `json:"openapi"`
}

// Info represents the OpenAPI info section.
type Info struct {
	Version string `json:"version"`
	Title   string `json:"title"`
}

// PathItem represents a single path in the OpenAPI specification.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
}

// Operation represents an OpenAPI operation.
type Operation struct {
	XOpenGate   *OpenGateExt `json:"x-opengate,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Description string       `json:"description,omitempty"`
	OperationID string       `json:"operationId,omitempty"`
}

// OpenGateExt represents the OpenGate-specific routing configuration extension.
type OpenGateExt struct {
	Type    string         `json:"type"`
	Options HandlerOptions `json:"options"`
}

// HandlerOptions represents the handler options.
type HandlerOptions struct {
	// URL is used for forward handler type
	URL string `json:"url,omitempty"`
	// Location is used for redirect handler type
	Location string `json:"location,omitempty"`
	// StatusCode is used for redirect handler type (e.g., 301, 302, 307, 308)
	StatusCode int `json:"status_code,omitempty"`
}

// Parser handles parsing of OpenAPI specifications.
type Parser struct{}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses an OpenAPI specification from a file and returns a list of routes.
func (p *Parser) ParseFile(filePath string) ([]route.Route, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	return p.Parse(data)
}

// Parse parses an OpenAPI specification from raw bytes and returns a list of routes.
func (p *Parser) Parse(data []byte) ([]route.Route, error) {
	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal spec: %w", err)
	}

	routes := make([]route.Route, 0)

	for path, pathItem := range spec.Paths {
		routes = append(routes, p.extractRoutes(path, pathItem)...)
	}

	return routes, nil
}

// extractRoutes extracts all routes from a PathItem.
func (p *Parser) extractRoutes(path string, pathItem PathItem) []route.Route {
	routes := make([]route.Route, 0)

	if pathItem.Get != nil {
		routes = append(routes, p.createRoute(path, "GET", pathItem.Get))
	}

	if pathItem.Post != nil {
		routes = append(routes, p.createRoute(path, "POST", pathItem.Post))
	}

	if pathItem.Put != nil {
		routes = append(routes, p.createRoute(path, "PUT", pathItem.Put))
	}

	if pathItem.Delete != nil {
		routes = append(routes, p.createRoute(path, "DELETE", pathItem.Delete))
	}

	if pathItem.Patch != nil {
		routes = append(routes, p.createRoute(path, "PATCH", pathItem.Patch))
	}

	return routes
}

// createRoute creates a Route from an Operation.
func (p *Parser) createRoute(path, method string, op *Operation) route.Route {
	r := route.Route{
		Path:        path,
		Method:      method,
		OperationID: op.OperationID,
	}

	if op.XOpenGate != nil {
		handler := route.Handler{
			Type: op.XOpenGate.Type,
		}

		switch op.XOpenGate.Type {
		case "forward":
			handler.BaseURL = op.XOpenGate.Options.URL
		case "redirect":
			handler.Location = op.XOpenGate.Options.Location
			handler.StatusCode = op.XOpenGate.Options.StatusCode
		}

		r.Handler = handler
	}

	return r
}
