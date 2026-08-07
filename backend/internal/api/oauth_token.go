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
// RFC 6749 §2.3.1: HTTP Basic auth is checked first (the RFC-preferred form
// for confidential clients), falling back to
// application/x-www-form-urlencoded body parameters -- r.ParseForm also
// covers a JSON-unaware client that just POSTs form-encoded fields, which
// is the common case for OAuth2 client libraries.
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
// 6749 §4.4) applications use to exchange a client_id/client_secret for a
// short-lived access token (see auth.Service.GenerateServiceToken). It is
// registered without requirePermission/withAudit -- like loginHandler --
// since the request itself carries the credential being authenticated;
// there is no prior bearer token to check.
//
// Registered on the method-agnostic "/api/oauth/token" pattern (see
// RegisterRoutes), not "POST /api/oauth/token" -- unlike every other route
// in this package, the method check happens here instead of being left to
// Go 1.22+ ServeMux, because a non-POST request must still get this
// endpoint's RFC 6749 JSON error body rather than ServeMux's generic
// plain-text 405.
//
// Both the request and response shapes deliberately follow RFC 6749
// verbatim instead of this API's usual JSON conventions (structured
// {"error","code"} responses, camelCase bodies elsewhere) -- that is the
// explicit point of this endpoint: a standard, unmodified OAuth2
// client-credentials client should be able to call it successfully.
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
