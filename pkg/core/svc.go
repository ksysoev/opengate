// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"fmt"

	"github.com/ksysoev/opengate/pkg/spec"
)

// specParser defines the interface for parsing OpenAPI specifications.
type specParser interface {
	ParseFile(filePath string) ([]spec.Route, error)
}

// Service encapsulates core business logic and dependencies.
type Service struct {
	parser specParser
	routes []spec.Route
}

// New creates a new Service instance with the provided dependencies.
func New(parser specParser) *Service {
	return &Service{
		parser: parser,
		routes: make([]spec.Route, 0),
	}
}

// LoadSpec loads routes from an OpenAPI specification file.
func (s *Service) LoadSpec(ctx context.Context, specPath string) error {
	routes, err := s.parser.ParseFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to parse spec file: %w", err)
	}

	s.routes = routes

	return nil
}

// GetRoutes returns all loaded routes.
func (s *Service) GetRoutes(ctx context.Context) []spec.Route {
	return s.routes
}
