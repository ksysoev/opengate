package core

import "fmt"

// Sentinel errors for common failure cases.
var (
	ErrRouteNotFound   = fmt.Errorf("route not found in context")
	ErrInvalidRoute    = fmt.Errorf("invalid route configuration")
	ErrBackendFailed   = fmt.Errorf("backend request failed")
	ErrBackendTimeout  = fmt.Errorf("backend request timeout")
	ErrInvalidRedirect = fmt.Errorf("invalid redirect configuration")
)

// BackendError wraps backend-specific errors with context.
type BackendError struct {
	Err        error  // Underlying error
	BackendURL string // Backend URL that failed
	StatusCode int    // HTTP status from backend (0 if no response)
}

func (e *BackendError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("backend %s returned status %d: %v", e.BackendURL, e.StatusCode, e.Err)
	}

	return fmt.Sprintf("backend %s failed: %v", e.BackendURL, e.Err)
}

func (e *BackendError) Unwrap() error {
	return e.Err
}

// RedirectError wraps redirect validation errors.
type RedirectError struct {
	Reason     string
	Location   string
	StatusCode int
}

func (e *RedirectError) Error() string {
	return fmt.Sprintf("redirect error: %s", e.Reason)
}
