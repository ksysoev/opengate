// Package redirect provides HTTP redirect functionality for the API gateway.
package redirect

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ksysoev/opengate/pkg/core"
	"github.com/ksysoev/opengate/pkg/core/request"
	"github.com/ksysoev/opengate/pkg/core/route"
)

// Redirector handles HTTP redirects.
type Redirector struct{}

// New creates a new redirect Redirector instance.
func New() *Redirector {
	return &Redirector{}
}

// Handle implements core.Handler interface for redirecting requests.
func (r *Redirector) Handle(ctx context.Context, req *request.Request, rt *route.Route) (*request.Response, error) {
	// Validate configuration
	if rt.Handler.Location == "" {
		return nil, &core.RedirectError{
			Reason: "no redirect location configured",
		}
	}

	if rt.Handler.StatusCode == 0 {
		return nil, &core.RedirectError{
			Reason: "no redirect status code configured",
		}
	}

	if !isValidRedirectStatus(rt.Handler.StatusCode) {
		return nil, &core.RedirectError{
			Reason:     fmt.Sprintf("invalid redirect status code: %d", rt.Handler.StatusCode),
			StatusCode: rt.Handler.StatusCode,
		}
	}

	// Build redirect response
	headers := make(http.Header)
	headers.Set("Location", rt.Handler.Location)

	return &request.Response{
		StatusCode: rt.Handler.StatusCode,
		Headers:    headers,
		Body:       http.NoBody,
	}, nil
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
