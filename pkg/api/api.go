// Package api provides the implementation of the API server for the application.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ksysoev/opengate/pkg/api/middleware"
	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
)

const (
	defaultTimeout = 30 * time.Second
)

// Service defines the interface for the core service that API depends on.
// This interface is defined on the consumer side (API) and only includes methods used by API.
type Service interface {
	HandleRequest(ctx context.Context, req *request.Request) (*request.Response, error)
}

type API struct {
	svc    Service
	config Config
}

type Config struct {
	Listen string `mapstructure:"listen"`
}

// New creates a new API instance with the provided configuration and core service.
// It validates the configuration.
func New(cfg Config, svc Service) (*API, error) {
	if cfg.Listen == "" {
		return nil, errors.New("listen address must be specified")
	}

	api := &API{
		config: cfg,
		svc:    svc,
	}

	return api, nil
}

// ServeHTTP implements http.Handler interface.
// It converts HTTP requests to core requests and delegates to the core service.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Convert HTTP request to core request
	coreReq, err := httpToCore(r, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to convert request", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	// Delegate to core service
	coreResp, err := a.svc.HandleRequest(r.Context(), coreReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "Request handling failed", "error", err,
			"method", r.Method, "path", r.URL.Path)
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
	case errors.Is(err, core.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, core.ErrForbidden):
		return http.StatusForbidden
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
