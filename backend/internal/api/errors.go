package api

import (
	"errors"
	"log"
	"net/http"

	"aerendil/backend/internal/store"
)

// apiError pairs an HTTP status and a stable machine-readable code (see
// error_codes.go) with a client-facing message. Handlers return one instead
// of writing a response directly, so handleErrors is the single place that
// turns it into JSON. message is English debug text; code is what the
// frontend translates.
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

// storeErrorToAPIError maps a known store-layer sentinel to the apiError
// handleErrors should write; nil means handleErrors should treat it as an
// unexpected internal error. Centralizes what every handler used to do
// inline, so a handler can just `return err`.
func storeErrorToAPIError(err error) *apiError {
	switch {
	case errors.Is(err, store.ErrUsernameTaken):
		return &apiError{http.StatusConflict, CodeConflictUsernameTaken, MsgConflictUsernameTaken}
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
// writing a failure response itself; handleErrors adapts it so it composes
// into the same chain as requirePermission/withAudit.
type apiHandlerFunc func(w http.ResponseWriter, r *http.Request) error

// handleErrors adapts h into an http.HandlerFunc. A nil return means h
// already wrote its own response. A non-nil return: *apiError writes
// verbatim, a recognized store sentinel maps via storeErrorToAPIError,
// anything else logs server-side and returns an opaque 500 so internal
// details never reach the client.
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

// writeError writes the {"error", "code"} JSON shape every handler uses --
// shared by handleErrors and the few call sites outside the apiHandlerFunc
// chain (requirePermission/authenticateRequest, withAudit).
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
