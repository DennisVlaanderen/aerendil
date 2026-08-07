package api

import (
	"errors"
	"log"
	"net/http"

	"aerendil/backend/internal/store"
)

// apiError pairs an HTTP status and a stable machine-readable code (see
// error_codes.go) with a client-facing message. Handlers return one (via
// badRequest/unauthorized/forbidden/notFound/conflict/methodNotAllowed/
// internalError below) instead of writing an error response directly --
// handleErrors is the single place that turns a returned error into the
// actual {"error": "...", "code": "..."} JSON response, so every handler's
// error path looks the same regardless of which failure it hit. message
// stays English debug text for logs/tools; code is what the frontend uses
// to pick a translated string, since message itself is never translated.
type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string { return e.message }

func badRequest(code, message string) error {
	return &apiError{http.StatusBadRequest, code, message}
}
func unauthorized(code, message string) error {
	return &apiError{http.StatusUnauthorized, code, message}
}
func forbidden(code, message string) error {
	return &apiError{http.StatusForbidden, code, message}
}
func notFound(code, message string) error {
	return &apiError{http.StatusNotFound, code, message}
}
func conflict(code, message string) error {
	return &apiError{http.StatusConflict, code, message}
}
func methodNotAllowed(code, message string) error {
	return &apiError{http.StatusMethodNotAllowed, code, message}
}
func internalError(code, message string) error {
	return &apiError{http.StatusInternalServerError, code, message}
}

// storeErrorToAPIError maps a known, client-facing store-layer sentinel to
// the apiError handleErrors should write; nil if err doesn't match any of
// them, meaning handleErrors should treat it as an unexpected internal
// error instead. This is the mapping every handler used to apply inline via
// writeStoreError; centralizing it here means a handler that fails a store
// call can just `return err` and let handleErrors sort out the status.
func storeErrorToAPIError(err error) *apiError {
	switch {
	case errors.Is(err, store.ErrUsernameTaken):
		return &apiError{http.StatusConflict, CodeConflictUsernameTaken, "username is already taken"}
	case errors.Is(err, store.ErrLastAdmin):
		return &apiError{http.StatusForbidden, CodeBusinessLastAdmin, err.Error()}
	case errors.Is(err, store.ErrProtectedSystemGroup):
		return &apiError{http.StatusForbidden, CodeBusinessProtectedGroup, err.Error()}
	case errors.Is(err, store.ErrLastEnvironment):
		return &apiError{http.StatusForbidden, CodeBusinessLastEnvironment, err.Error()}
	case errors.Is(err, store.ErrEnvironmentHasFlags):
		return &apiError{http.StatusForbidden, CodeBusinessEnvironmentHasFlags, err.Error()}
	case errors.Is(err, store.ErrEnvironmentHasCredentials):
		return &apiError{http.StatusForbidden, CodeBusinessEnvironmentHasCredentials, err.Error()}
	case errors.Is(err, store.ErrUnknownEnvironment):
		return &apiError{http.StatusBadRequest, CodeBadRequestEnvironmentUnknown, err.Error()}
	default:
		return nil
	}
}

// apiHandlerFunc is like http.HandlerFunc but returns an error instead of
// writing a failure response itself -- handleErrors adapts one into a real
// http.HandlerFunc, so it composes into the existing middleware chain the
// same way requirePermission/withAudit already do:
// requirePermission(perm, withAudit(cfg, handleErrors(handler))).
type apiHandlerFunc func(w http.ResponseWriter, r *http.Request) error

// handleErrors adapts h into an http.HandlerFunc. On a nil return, h is
// assumed to have already written its own success response (via writeJSON)
// and nothing more happens. On a non-nil return: an *apiError writes its
// status/message verbatim; a recognized store-layer sentinel (see
// storeErrorToAPIError) is mapped to its client-facing status/message;
// anything else is logged server-side and returned as an opaque 500 so
// internal error text (paths, raft/bolt internals, etc.) never reaches the
// client.
func handleErrors(h apiHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}

		var apiErr *apiError
		if errors.As(err, &apiErr) {
			writeError(w, apiErr.status, apiErr.code, apiErr.message)
			return
		}
		if mapped := storeErrorToAPIError(err); mapped != nil {
			writeError(w, mapped.status, mapped.code, mapped.message)
			return
		}
		log.Printf("api: internal error: %v", err)
		writeError(w, http.StatusInternalServerError, CodeInternalGeneric, "internal server error")
	}
}

// writeError writes the {"error": "...", "code": "..."} JSON shape every
// handler's error path uses -- shared by handleErrors and the small number
// of call sites that write an error response outside the apiHandlerFunc
// chain entirely (requirePermission/authenticateRequest in middleware.go
// run before a handler is ever reached; withAudit in audit_middleware.go
// wraps the chain's outermost response write).
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
