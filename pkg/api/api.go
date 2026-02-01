// Package api provides the implementation of the API server for the application.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ksysoev/opengate/pkg/api/middleware"
	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/proxy"
	"github.com/ksysoev/opengate/pkg/core/redirect"
	"github.com/ksysoev/opengate/pkg/core/router"
)

const (
	defaultTimeout = 30 * time.Second
)

type API struct {
	router          *router.Router
	proxyHandler    core.Handler
	redirectHandler core.Handler
	config          Config
}

type Config struct {
	Listen string `mapstructure:"listen"`
}

// New creates a new API instance with the provided configuration and core service.
// It validates the configuration, loads routes from the service, and sets up the router.
// Returns an error if the listen address is not specified or if routes cannot be loaded.
func New(cfg Config, svc *core.Service) (*API, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address must be specified")
	}

	// Get routes from core service
	routes := svc.GetRoutes(context.Background())
	if len(routes) == 0 {
		return nil, fmt.Errorf("no routes loaded from spec")
	}

	// Create router and register routes
	rtr := router.New()
	for i := range routes {
		if err := rtr.AddRoute(&routes[i]); err != nil {
			return nil, fmt.Errorf("failed to add route: %w", err)
		}

		// Type-aware logging
		switch routes[i].Handler.Type {
		case "forward":
			slog.Debug("Registered forward route",
				"method", routes[i].Method,
				"path", routes[i].Path,
				"backend", routes[i].Handler.BaseURL,
				"operation_id", routes[i].OperationID)
		case "redirect":
			slog.Debug("Registered redirect route",
				"method", routes[i].Method,
				"path", routes[i].Path,
				"location", routes[i].Handler.Location,
				"status_code", routes[i].Handler.StatusCode,
				"operation_id", routes[i].OperationID)
		default:
			slog.Debug("Registered route",
				"method", routes[i].Method,
				"path", routes[i].Path,
				"type", routes[i].Handler.Type,
				"operation_id", routes[i].OperationID)
		}
	}

	api := &API{
		config:          cfg,
		router:          rtr,
		proxyHandler:    proxy.New(),
		redirectHandler: redirect.New(),
	}

	return api, nil
}

// ServeHTTP implements http.Handler interface.
// It routes incoming HTTP requests to the appropriate handler based on the registered routes.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Match the route
	route, params, err := a.router.Match(r.Method, r.URL.Path)
	if err != nil {
		slog.DebugContext(r.Context(), "No matching route",
			"method", r.Method,
			"path", r.URL.Path)
		http.Error(w, "Not Found", http.StatusNotFound)

		return
	}

	// Convert HTTP request to core request
	coreReq, err := httpToCore(r, params)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to convert request", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	// Select and call the appropriate handler
	var handler core.Handler

	switch route.Handler.Type {
	case "forward":
		handler = a.proxyHandler
	case "redirect":
		handler = a.redirectHandler
	default:
		slog.ErrorContext(r.Context(), "Unknown handler type",
			"type", route.Handler.Type,
			"path", route.Path,
			"method", route.Method)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	// Call core handler
	coreResp, err := handler.Handle(r.Context(), coreReq, route)
	if err != nil {
		slog.ErrorContext(r.Context(), "Handler failed", "error", err,
			"path", route.Path, "method", route.Method, "handler_type", route.Handler.Type)
		status := errorToHTTPStatus(err)
		http.Error(w, http.StatusText(status), status)

		return
	}

	// Convert core response to HTTP response
	if err := coreToHTTP(w, coreResp); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
		// Response may be partially written, but log the error
	}
}

// Run starts the API server with the provided configuration.
// It listens on the address specified in the configuration and handles graceful shutdown.
// The server will log any errors encountered during shutdown.
// If the server fails to start, it returns an error.
func (a *API) Run(ctx context.Context) error {
	// Build middleware chain
	withReqID := middleware.NewReqID()
	handler := withReqID(a)

	s := &http.Server{
		Addr:              a.config.Listen,
		ReadHeaderTimeout: defaultTimeout,
		WriteTimeout:      defaultTimeout,
		Handler:           handler,
	}

	go func() {
		<-ctx.Done()

		err := s.Close()

		slog.WarnContext(ctx, "shutting down API server", "error", err)
	}()

	slog.Info("Starting API gateway", "addr", a.config.Listen)

	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

// errorToHTTPStatus maps core errors to HTTP status codes.
func errorToHTTPStatus(err error) int {
	switch {
	case errors.Is(err, core.ErrRouteNotFound):
		return http.StatusNotFound
	case errors.Is(err, core.ErrInvalidRoute):
		return http.StatusInternalServerError
	case errors.Is(err, core.ErrInvalidRedirect):
		return http.StatusInternalServerError
	case errors.Is(err, core.ErrBackendTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, core.ErrBackendFailed):
		var backendErr *core.BackendError
		if errors.As(err, &backendErr) && backendErr.StatusCode > 0 {
			// Pass through backend status code
			return backendErr.StatusCode
		}

		return http.StatusBadGateway
	default:
		// Check for RedirectError
		var redirectErr *core.RedirectError
		if errors.As(err, &redirectErr) {
			return http.StatusInternalServerError
		}

		return http.StatusInternalServerError
	}
}
