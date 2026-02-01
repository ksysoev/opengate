// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
	"github.com/ksysoev/opengate/pkg/core/router"
)

// specParser defines the interface for parsing OpenAPI specifications.
type specParser interface {
	ParseFile(filePath string) ([]route.Route, error)
}

// Service encapsulates core business logic and dependencies.
type Service struct {
	parser          specParser
	matcher         *router.Matcher
	handlers        map[string]Handler
	middlewareChain *middleware.Chain
	routes          []route.Route
}

// New creates a new Service instance with the provided dependencies.
func New(parser specParser) *Service {
	return &Service{
		parser:          parser,
		matcher:         router.New(),
		handlers:        make(map[string]Handler),
		routes:          make([]route.Route, 0),
		middlewareChain: nil, // Will be set via SetMiddlewareChain
	}
}

// RegisterHandler registers a handler for a specific handler type.
func (s *Service) RegisterHandler(handlerType string, handler Handler) {
	s.handlers[handlerType] = handler
}

// SetMiddlewareChain sets the middleware chain for the service.
func (s *Service) SetMiddlewareChain(chain *middleware.Chain) {
	s.middlewareChain = chain
}

// LoadSpec loads routes from an OpenAPI specification file and registers them with the router.
func (s *Service) LoadSpec(ctx context.Context, specPath string) error {
	routes, err := s.parser.ParseFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to parse spec file: %w", err)
	}

	if len(routes) == 0 {
		return fmt.Errorf("no routes found in spec file")
	}

	// Reset matcher and routes to prevent duplicates on reload
	s.matcher = router.New()
	s.routes = routes

	// Register routes with matcher
	for i := range s.routes {
		if err := s.matcher.AddRoute(&s.routes[i]); err != nil {
			return fmt.Errorf("failed to add route: %w", err)
		}

		// Type-aware logging
		switch s.routes[i].Handler.Type {
		case "forward":
			slog.Debug("Registered forward route",
				"method", s.routes[i].Method,
				"path", s.routes[i].Path,
				"backend", s.routes[i].Handler.BaseURL,
				"operation_id", s.routes[i].OperationID)
		case "redirect":
			slog.Debug("Registered redirect route",
				"method", s.routes[i].Method,
				"path", s.routes[i].Path,
				"location", s.routes[i].Handler.Location,
				"status_code", s.routes[i].Handler.StatusCode,
				"operation_id", s.routes[i].OperationID)
		default:
			slog.Debug("Registered route",
				"method", s.routes[i].Method,
				"path", s.routes[i].Path,
				"type", s.routes[i].Handler.Type,
				"operation_id", s.routes[i].OperationID)
		}
	}

	return nil
}

// HandleRequest processes a request by routing it to the appropriate handler.
func (s *Service) HandleRequest(ctx context.Context, req *request.Request) (*request.Response, error) {
	// Match the route
	rt, params, err := s.matcher.Match(req.Method, req.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s", ErrRouteNotFound, req.Method, req.Path)
	}

	// Update path params in request
	req.PathParams = params

	// Get handler for route type
	handler, ok := s.handlers[rt.Handler.Type]
	if !ok {
		return nil, fmt.Errorf("%w: no handler registered for type %s", ErrInvalidRoute, rt.Handler.Type)
	}

	// Build final handler function
	finalHandler := func(ctx context.Context, req *request.Request) (*request.Response, error) {
		return handler.Handle(ctx, req, rt)
	}

	// If middleware chain is set and route has policies, execute through chain
	if s.middlewareChain != nil && len(rt.Policies) > 0 {
		chainedHandler := s.middlewareChain.Build(rt.Policies, finalHandler)
		return chainedHandler(ctx, req)
	}

	// No middleware, call handler directly
	return finalHandler(ctx, req)
}

// GetRoutes returns all loaded routes.
// Returns a copy to prevent external mutation of internal state.
func (s *Service) GetRoutes(ctx context.Context) []route.Route {
	routes := make([]route.Route, len(s.routes))
	copy(routes, s.routes)

	return routes
}
