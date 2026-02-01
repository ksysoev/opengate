package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/opengate/pkg/api"
	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/middleware"
	"github.com/ksysoev/opengate/pkg/core/proxy"
	"github.com/ksysoev/opengate/pkg/core/redirect"
	"github.com/ksysoev/opengate/pkg/middleware/oidc"
	httpprov "github.com/ksysoev/opengate/pkg/prov/http"
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

	// Create HTTP provider
	httpClient, err := httpprov.New(cfg.HTTP)
	if err != nil {
		return fmt.Errorf("failed to create HTTP provider: %w", err)
	}

	// Create runtime with providers
	runtime, err := core.NewRuntime(httpClient)
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	// Create core service
	parser := spec.NewParser()
	svc := core.New(parser)

	// Register handlers with runtime
	forwarder, err := proxy.New(runtime)
	if err != nil {
		return fmt.Errorf("failed to create forwarder: %w", err)
	}

	svc.RegisterHandler("forward", forwarder)
	svc.RegisterHandler("redirect", redirect.New())

	// Register middleware factories (only if not already registered)
	if !middleware.GetRegistry().HasFactory("oidc") {
		if err := middleware.Register("oidc", oidc.Create); err != nil {
			return fmt.Errorf("failed to register oidc middleware: %w", err)
		}
	}

	// Convert policy config to policies map
	policies := cfg.Policies.ToPolicies()

	// Initialize middleware registry with runtime and policies
	if err := middleware.Initialize(runtime, policies); err != nil {
		return fmt.Errorf("failed to initialize middleware: %w", err)
	}

	// Load OpenAPI specification
	if err := svc.LoadSpec(ctx, cfg.Gateway.SpecPath); err != nil {
		return fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	routes := svc.GetRoutes(ctx)

	// Validate and preload all policies referenced by routes
	for i := range routes {
		if len(routes[i].Policies) > 0 {
			if err := middleware.PreloadPolicies(routes[i].Policies); err != nil {
				return fmt.Errorf("policy preload failed for route %s %s: %w", routes[i].Method, routes[i].Path, err)
			}
		}
	}

	slog.Info("Loaded routes from OpenAPI spec", "count", len(routes))

	// Create API service - it delegates to core service
	apiSvc, err := api.New(cfg.API, svc)
	if err != nil {
		return fmt.Errorf("failed to create API service: %w", err)
	}

	// Run the API server
	err = apiSvc.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to run API service: %w", err)
	}

	return nil
}
