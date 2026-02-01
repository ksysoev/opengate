package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/route"
)

// HandlerAdapter adapts a core.Handler to work with HTTP requests.
type HandlerAdapter struct {
	coreHandler core.Handler
}

// NewHandlerAdapter creates a new HTTP adapter for a core handler.
func NewHandlerAdapter(handler core.Handler) *HandlerAdapter {
	return &HandlerAdapter{coreHandler: handler}
}

// ServeHTTP implements http.Handler by adapting to core.Handler.
// It converts HTTP requests to core requests, calls the core handler,
// and converts the core response back to HTTP.
func (a *HandlerAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request, rt *route.Route, pathParams map[string]string) {
	// Convert HTTP request to core request
	coreReq, err := HTTPToCore(r, pathParams)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to convert request", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	// Call core handler
	coreResp, err := a.coreHandler.Handle(r.Context(), coreReq, rt)
	if err != nil {
		slog.ErrorContext(r.Context(), "Handler failed", "error", err,
			"path", rt.Path, "method", rt.Method, "handler_type", rt.Handler.Type)
		status := errorToHTTPStatus(err)
		http.Error(w, http.StatusText(status), status)

		return
	}

	// Convert core response to HTTP response
	if err := CoreToHTTP(w, coreResp); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
		// Response may be partially written, but log the error
	}
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
