package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ksysoev/opengate/pkg/api"
	"github.com/ksysoev/opengate/pkg/api/middleware"
	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/proxy"
	"github.com/ksysoev/opengate/pkg/core/redirect"
	"github.com/ksysoev/opengate/pkg/core/router"
	"github.com/ksysoev/opengate/pkg/spec"
)

// RunCommand initializes the logger, loads configuration, creates the core and API services,
// and starts the API service. It returns an error if any step fails.
func RunCommand(ctx context.Context, flags *cmdFlags) error {
	if err := initLogger(flags); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(flags)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Gateway.SpecPath == "" {
		return fmt.Errorf("gateway spec path must be specified")
	}

	// Create dependencies
	parser := spec.NewParser()
	svc := core.New(parser)

	// Load OpenAPI specification
	if err := svc.LoadSpec(ctx, cfg.Gateway.SpecPath); err != nil {
		return fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	routes := svc.GetRoutes(ctx)
	slog.Info("Loaded routes from OpenAPI spec", "count", len(routes))

	// Create router and register routes
	rtr := router.New()
	for i := range routes {
		if err := rtr.AddRoute(&routes[i]); err != nil {
			return fmt.Errorf("failed to add route: %w", err)
		}

		slog.Debug("Registered route",
			"method", routes[i].Method,
			"path", routes[i].Path,
			"backend", routes[i].Handler.BaseURL,
			"operation_id", routes[i].OperationID)
	}

	// Create handlers
	proxyHandler := proxy.New()
	redirectHandler := redirect.New()

	// Build middleware chain
	withReqID := middleware.NewReqID()

	handler := withReqID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First, match the route
		route, params, err := rtr.Match(r.Method, r.URL.Path)
		if err != nil {
			slog.DebugContext(r.Context(), "No matching route",
				"method", r.Method,
				"path", r.URL.Path)
			http.Error(w, "Not Found", http.StatusNotFound)

			return
		}

		// Store route and params in context
		ctx := r.Context()
		for key, value := range params {
			ctx = router.WithPathParam(ctx, key, value)
		}

		ctx = router.WithRoute(ctx, route)

		// Route to appropriate handler based on type
		switch route.Handler.Type {
		case "forward":
			proxyHandler.ServeHTTP(w, r.WithContext(ctx))
		case "redirect":
			redirectHandler.ServeHTTP(w, r.WithContext(ctx))
		default:
			slog.ErrorContext(ctx, "Unknown handler type",
				"type", route.Handler.Type,
				"path", route.Path,
				"method", route.Method)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}))

	apiSvc, err := api.New(cfg.API, svc)
	if err != nil {
		return fmt.Errorf("failed to create API service: %w", err)
	}

	apiSvc.SetMux(handler)

	err = apiSvc.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to run API service: %w", err)
	}

	return nil
}
