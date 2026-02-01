package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/opengate/pkg/api"
	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/proxy"
	"github.com/ksysoev/opengate/pkg/core/redirect"
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

	// Create core service
	parser := spec.NewParser()
	svc := core.New(parser)

	// Register handlers with core service
	svc.RegisterHandler("forward", proxy.New())
	svc.RegisterHandler("redirect", redirect.New())

	// Load OpenAPI specification
	if err := svc.LoadSpec(ctx, cfg.Gateway.SpecPath); err != nil {
		return fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	routes := svc.GetRoutes(ctx)
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
