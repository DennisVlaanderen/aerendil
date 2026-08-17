package api

import (
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
)

// oauthErrorResponse follows RFC 6749 §5.2 verbatim ({"error",
// "error_description"}), not this API's usual {"error","code"} shape -- see
// oauthTokenHandler's doc comment for why.
type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, oauthErrorResponse{Error: code, ErrorDescription: description})
}

// parseOAuthTokenRequest extracts client_id/client_secret/grant_type per
// RFC 6749 §2.3.1: HTTP Basic auth first (RFC-preferred), falling back to
// form-urlencoded body params, the common case for OAuth2 client libraries.
func parseOAuthTokenRequest(r *http.Request) (clientID, clientSecret, grantType string) {
	if id, secret, ok := r.BasicAuth(); ok {
		clientID, clientSecret = strings.TrimSpace(id), strings.TrimSpace(secret)
	}
	_ = r.ParseForm()
	if clientID == "" {
		clientID = strings.TrimSpace(r.PostFormValue("client_id"))
	}
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(r.PostFormValue("client_secret"))
	}
	grantType = strings.TrimSpace(r.PostFormValue("grant_type"))
	return clientID, clientSecret, grantType
}

// oauthTokenHandler implements the OAuth2 client-credentials grant (RFC
// 6749 §4.4): exchanges client_id/client_secret for a short-lived access
// token. Unprotected like loginHandler -- the request carries its own
// credential.
//
// Registered on the method-agnostic "/api/oauth/token" pattern, not "POST
// /api/oauth/token", so a non-POST request gets this endpoint's RFC 6749
// JSON error body instead of ServeMux's plain-text 405.
//
// Request and response shapes follow RFC 6749 verbatim, not this API's
// usual {"error","code"}/camelCase conventions -- an unmodified OAuth2
// client should just work against it.
func oauthTokenHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return nil
	}

	clientID, clientSecret, grantType := parseOAuthTokenRequest(r)
	if grantType != "client_credentials" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "only grant_type=client_credentials is supported")
		return nil
	}
	if clientID == "" || clientSecret == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id and client_secret are required")
		return nil
	}

	cred, err := authService.AuthenticateClientCredentials(clientID, clientSecret)
	if err != nil {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "invalid client_id or client_secret")
		return nil
	}

	token, err := authService.GenerateServiceToken(cred)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to issue access token")
		return nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(auth.ServiceTokenTTL.Seconds()),
		"scope":        strings.Join(cred.Scopes, " "),
	})
	return nil
}
