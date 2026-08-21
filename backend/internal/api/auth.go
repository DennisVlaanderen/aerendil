package api

import "net/http"

func registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", handleErrors(loginHandler))
	mux.HandleFunc("/api/auth/me", handleErrors(meHandler))
}

func loginHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return methodNotAllowed(CodeMethodNotAllowed, "method not allowed")
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	user, err := authService.Authenticate(payload.Username, payload.Password)
	if err != nil {
		return unauthorized(CodeAuthInvalidCredentials, "invalid username or password")
	}

	token, err := authService.GenerateToken(user)
	if err != nil {
		return internalError(CodeInternalTokenGen, "failed to create token")
	}

	return ok(w, map[string]any{
		"token": token,
		"user":  user,
	})
}

func meHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return methodNotAllowed(CodeMethodNotAllowed, "method not allowed")
	}

	principal, found := authenticateRequest(w, r)
	if !found {
		return nil
	}

	return ok(w, map[string]any{
		"user":         principal.User,
		"isAdmin":      principal.IsAdmin,
		"permissions":  principal.Perms.Keys(),
		"environments": resolveEnvironmentSummaries(principal),
	})
}
