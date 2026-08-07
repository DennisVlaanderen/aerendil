package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aerendil/backend/internal/auth"
)

func TestOAuthTokenClientCredentialsGrantSucceedsWithBodyCredentials(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	adminToken := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate)
	clientID, clientSecret := createApplicationCredentialForTest(t, mux, adminToken, "billing-service", envID, []string{auth.PermFlagsRead})

	body := "grant_type=client_credentials&client_id=" + clientID + "&client_secret=" + clientSecret
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected token grant to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"].(string) == "" {
		t.Fatalf("expected a non-empty access_token, got %+v", resp)
	}
	if resp["token_type"] != "Bearer" {
		t.Fatalf("expected token_type Bearer, got %+v", resp)
	}
	if resp["expires_in"] == nil {
		t.Fatalf("expected expires_in to be set, got %+v", resp)
	}
	if resp["scope"] != auth.PermFlagsRead {
		t.Fatalf("expected scope %q, got %+v", auth.PermFlagsRead, resp)
	}
}

func TestOAuthTokenClientCredentialsGrantSucceedsWithBasicAuth(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	adminToken := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate)
	clientID, clientSecret := createApplicationCredentialForTest(t, mux, adminToken, "billing-service", envID, []string{auth.PermFlagsRead})

	body := "grant_type=client_credentials"
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected token grant via Basic auth to succeed, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthTokenRejectsWrongClientSecret(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	adminToken := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate)
	clientID, _ := createApplicationCredentialForTest(t, mux, adminToken, "billing-service", envID, []string{auth.PermFlagsRead})

	body := "grant_type=client_credentials&client_id=" + clientID + "&client_secret=wrong"
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a wrong client secret, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"] != "invalid_client" {
		t.Fatalf("expected RFC 6749 invalid_client error, got %+v", resp)
	}
}

func TestOAuthTokenRejectsUnsupportedGrantType(t *testing.T) {
	mux := newTestMux(t)

	body := "grant_type=password&client_id=x&client_secret=y"
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unsupported grant_type, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"] != "unsupported_grant_type" {
		t.Fatalf("expected RFC 6749 unsupported_grant_type error, got %+v", resp)
	}
}

// TestOAuthTokenIssuedAccessTokenWorksOverHTTPButOnlyWithinScope is the
// end-to-end proof of the "reuse the existing auth flow" design: the token
// this endpoint issues authenticates against the ordinary /api/flags route
// via the exact same requirePermission/AuthenticateToken path a human JWT
// uses, but is rejected for anything outside the credential's own scopes
// (here, flags:read only -- no users:* access at all).
func TestOAuthTokenIssuedAccessTokenWorksOverHTTPButOnlyWithinScope(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	adminToken := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate, auth.PermFlagsRead)
	clientID, clientSecret := createApplicationCredentialForTest(t, mux, adminToken, "billing-service", envID, []string{auth.PermFlagsRead})

	tokenBody := "grant_type=client_credentials&client_id=" + clientID + "&client_secret=" + clientSecret
	tokenReq := httptest.NewRequest(http.MethodPost, "/api/oauth/token", bytes.NewBufferString(tokenBody))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRec := httptest.NewRecorder()
	mux.ServeHTTP(tokenRec, tokenReq)
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("expected token grant to succeed, got %d: %s", tokenRec.Code, tokenRec.Body.String())
	}
	var tokenResp map[string]any
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	accessToken := tokenResp["access_token"].(string)

	flagsReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	flagsReq.Header.Set("Authorization", "Bearer "+accessToken)
	flagsRec := httptest.NewRecorder()
	mux.ServeHTTP(flagsRec, flagsReq)
	if flagsRec.Code != http.StatusOK {
		t.Fatalf("expected the service access token to authenticate against /api/flags, got %d: %s", flagsRec.Code, flagsRec.Body.String())
	}

	usersReq := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	usersReq.Header.Set("Authorization", "Bearer "+accessToken)
	usersRec := httptest.NewRecorder()
	mux.ServeHTTP(usersRec, usersReq)
	if usersRec.Code != http.StatusForbidden {
		t.Fatalf("expected the service access token to be forbidden from /api/users (out of scope), got %d: %s", usersRec.Code, usersRec.Body.String())
	}
}
