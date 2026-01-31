// Package redirect provides HTTP redirect functionality for the API gateway.
package redirect

import (
	"log/slog"
	"net/http"

	"github.com/ksysoev/opengate/pkg/core/router"
)

// Handler handles HTTP redirects.
type Handler struct{}

// New creates a new redirect Handler instance.
func New() *Handler {
	return &Handler{}
}

// ServeHTTP implements http.Handler interface for redirecting requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := router.GetRoute(r.Context())
	if route == nil {
		slog.ErrorContext(r.Context(), "No route found in context")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if route.Handler.Location == "" {
		slog.ErrorContext(r.Context(), "No redirect location configured for route",
			"path", route.Path, "method", route.Method)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if route.Handler.StatusCode == 0 {
		slog.ErrorContext(r.Context(), "No redirect status code configured for route",
			"path", route.Path, "method", route.Method)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	// Validate status code is a valid redirect code
	if !isValidRedirectStatus(route.Handler.StatusCode) {
		slog.ErrorContext(r.Context(), "Invalid redirect status code",
			"path", route.Path, "method", route.Method, "status_code", route.Handler.StatusCode)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	slog.DebugContext(r.Context(), "Redirecting request",
		"location", route.Handler.Location,
		"status_code", route.Handler.StatusCode)

	http.Redirect(w, r, route.Handler.Location, route.Handler.StatusCode)
}

// isValidRedirectStatus checks if the status code is a valid HTTP redirect status.
func isValidRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, // 301
		http.StatusFound,             // 302
		http.StatusSeeOther,          // 303
		http.StatusTemporaryRedirect, // 307
		http.StatusPermanentRedirect: // 308
		return true
	default:
		return false
	}
}
