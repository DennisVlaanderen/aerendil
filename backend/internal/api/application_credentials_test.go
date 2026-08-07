package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aerendil/backend/internal/auth"
)

// createApplicationCredentialForTest posts a real create request through
// the mux (not directly against dataStore) so these tests exercise the same
// validation/secret-generation path a real client hits. Returns the created
// credential's id and its one-time plaintext clientSecret.
func createApplicationCredentialForTest(t *testing.T, mux *http.ServeMux, token, name, environmentID string, scopes []string) (id, clientSecret string) {
	t.Helper()

	body, _ := json.Marshal(map[string]any{
		"name":          name,
		"environmentId": environmentID,
		"scopes":        scopes,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/application-credentials", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected create to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	secret, ok := created["clientSecret"].(string)
	if !ok || secret == "" {
		t.Fatalf("expected create response to include a plaintext clientSecret, got %+v", created)
	}
	return created["id"].(string), secret
}

func TestApplicationCredentialsFullCRUD(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate,
		auth.PermApplicationCredentialsRead, auth.PermApplicationCredentialsUpdate, auth.PermApplicationCredentialsDelete)

	id, secret := createApplicationCredentialForTest(t, mux, token, "billing-service", envID, []string{auth.PermFlagsRead})
	if secret == "" {
		t.Fatal("expected a non-empty client secret")
	}

	updateBody, _ := json.Marshal(map[string]any{
		"name":   "billing-service-renamed",
		"scopes": []string{auth.PermFlagsRead, auth.PermFlagsWrite},
		"active": true,
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/application-credentials/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
	var updated map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if _, hasSecret := updated["clientSecret"]; hasSecret {
		t.Fatal("expected update response to never include clientSecret")
	}
	if updated["name"] != "billing-service-renamed" {
		t.Fatalf("expected renamed credential, got %+v", updated)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/application-credentials?environmentId="+envID, nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list to succeed, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if bytes.Contains(listRec.Body.Bytes(), []byte("clientSecret")) {
		t.Fatal("expected list response to never include clientSecret")
	}
	if !bytes.Contains(listRec.Body.Bytes(), []byte("billing-service-renamed")) {
		t.Fatalf("expected renamed credential in list, got %s", listRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/application-credentials/"+id, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete to succeed, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestApplicationCredentialsPutLeavesOmittedFieldsUnchanged(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate,
		auth.PermApplicationCredentialsRead, auth.PermApplicationCredentialsUpdate)

	id, _ := createApplicationCredentialForTest(t, mux, token, "billing-service", envID, []string{auth.PermFlagsRead})

	// Omit both "scopes" and "active" entirely -- a client renaming a
	// credential this way must not silently deactivate it or wipe its
	// scopes (see the pointer fields on the PUT payload in
	// application_credentials.go).
	updateBody, _ := json.Marshal(map[string]any{"name": "billing-service-renamed"})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/application-credentials/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var updated struct {
		Active bool     `json:"active"`
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if !updated.Active {
		t.Fatal("expected omitting \"active\" to leave the credential active")
	}
	if len(updated.Scopes) != 1 || updated.Scopes[0] != auth.PermFlagsRead {
		t.Fatalf("expected omitting \"scopes\" to leave the original scopes untouched, got %+v", updated.Scopes)
	}
}

func TestApplicationCredentialsPutCannotReassignEnvironment(t *testing.T) {
	mux := newTestMux(t)
	envA := seedEnvironmentForTest(t, "Production")
	envB := seedEnvironmentForTest(t, "Staging")
	token := tokenForWithEnvironments(t, []string{envA, envB}, auth.PermApplicationCredentialsCreate,
		auth.PermApplicationCredentialsUpdate)

	id, _ := createApplicationCredentialForTest(t, mux, token, "billing-service", envA, nil)

	updateBody, _ := json.Marshal(map[string]any{"name": "billing-service", "environmentId": envB})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/application-credentials/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var updated struct {
		EnvironmentID string `json:"environmentId"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.EnvironmentID != envA {
		t.Fatalf("expected PUT to ignore a changed environmentId and keep %q, got %q", envA, updated.EnvironmentID)
	}
}

func TestApplicationCredentialsPostRejectsUnknownScope(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate)

	body, _ := json.Marshal(map[string]any{
		"name":          "billing-service",
		"environmentId": envID,
		"scopes":        []string{"users:delete"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/application-credentials", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-credential scope, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApplicationCredentialsPostRejectsUnknownEnvironment(t *testing.T) {
	mux := newTestMux(t)
	// Grant access to the made-up ID itself (mirroring
	// TestFlagsPostRejectsUnknownEnvironmentWithNoPartialWrites in
	// flags_test.go) so the environment-access check passes and the failure
	// comes from environment *existence* instead -- isolating the 400 case
	// from the 403 case covered by TestApplicationCredentialsPostRejectsInaccessibleEnvironment.
	token := tokenForWithEnvironments(t, []string{"does-not-exist"}, auth.PermApplicationCredentialsCreate)

	body, _ := json.Marshal(map[string]any{
		"name":          "billing-service",
		"environmentId": "does-not-exist",
		"scopes":        []string{auth.PermFlagsRead},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/application-credentials", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown environment, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestApplicationCredentialsPostRejectsInaccessibleEnvironment guards the
// environment-scoping gap application_credentials.go used to have: a group
// restricted to one environment must not be able to create a credential
// scoped to a different one, even while holding
// applicationCredentials:create.
func TestApplicationCredentialsPostRejectsInaccessibleEnvironment(t *testing.T) {
	mux := newTestMux(t)
	staging := seedEnvironmentForTest(t, "Staging")
	production := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{staging}, auth.PermApplicationCredentialsCreate)

	body, _ := json.Marshal(map[string]any{
		"name":          "billing-service",
		"environmentId": production,
		"scopes":        []string{auth.PermFlagsRead},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/application-credentials", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 creating a credential in an inaccessible environment, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestApplicationCredentialsGetRejectsInaccessibleEnvironment guards the
// same gap on the list route: a group without access to an environment must
// not be able to list its credentials by simply naming it in the query.
func TestApplicationCredentialsGetRejectsInaccessibleEnvironment(t *testing.T) {
	mux := newTestMux(t)
	staging := seedEnvironmentForTest(t, "Staging")
	production := seedEnvironmentForTest(t, "Production")
	adminToken := tokenForWithEnvironments(t, []string{production}, auth.PermApplicationCredentialsCreate)
	createApplicationCredentialForTest(t, mux, adminToken, "billing-service", production, nil)

	restrictedToken := tokenForWithEnvironments(t, []string{staging}, auth.PermApplicationCredentialsRead)
	req := httptest.NewRequest(http.MethodGet, "/api/application-credentials?environmentId="+production, nil)
	req.Header.Set("Authorization", "Bearer "+restrictedToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 listing credentials in an inaccessible environment, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestApplicationCredentialsMutationsRejectInaccessibleEnvironment guards
// PUT/DELETE/rotate: a group with the right permission but no access to the
// credential's own environment must be rejected on all three routes.
func TestApplicationCredentialsMutationsRejectInaccessibleEnvironment(t *testing.T) {
	mux := newTestMux(t)
	staging := seedEnvironmentForTest(t, "Staging")
	production := seedEnvironmentForTest(t, "Production")
	adminToken := tokenForWithEnvironments(t, []string{production}, auth.PermApplicationCredentialsCreate)
	id, _ := createApplicationCredentialForTest(t, mux, adminToken, "billing-service", production, nil)

	restrictedToken := tokenForWithEnvironments(t, []string{staging},
		auth.PermApplicationCredentialsUpdate, auth.PermApplicationCredentialsDelete)

	updateBody, _ := json.Marshal(map[string]any{"name": "renamed"})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/application-credentials/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+restrictedToken)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 updating a credential in an inaccessible environment, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	rotateReq := httptest.NewRequest(http.MethodPost, "/api/application-credentials/"+id+"/rotate", nil)
	rotateReq.Header.Set("Authorization", "Bearer "+restrictedToken)
	rotateRec := httptest.NewRecorder()
	mux.ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 rotating a credential in an inaccessible environment, got %d: %s", rotateRec.Code, rotateRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/application-credentials/"+id, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+restrictedToken)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting a credential in an inaccessible environment, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestApplicationCredentialsPutAndDeleteReturnNotFoundForUnknownID(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermApplicationCredentialsUpdate, auth.PermApplicationCredentialsDelete)

	updateBody, _ := json.Marshal(map[string]any{"name": "x", "scopes": []string{}})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/application-credentials/does-not-exist", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 updating an unknown credential, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/application-credentials/does-not-exist", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 deleting an unknown credential, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestApplicationCredentialsRotateIssuesNewSecretAndInvalidatesOld(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate, auth.PermApplicationCredentialsUpdate)

	id, originalSecret := createApplicationCredentialForTest(t, mux, token, "billing-service", envID, []string{auth.PermFlagsRead})

	rotateReq := httptest.NewRequest(http.MethodPost, "/api/application-credentials/"+id+"/rotate", nil)
	rotateReq.Header.Set("Authorization", "Bearer "+token)
	rotateRec := httptest.NewRecorder()
	mux.ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("expected rotate to succeed, got %d: %s", rotateRec.Code, rotateRec.Body.String())
	}
	var rotated map[string]any
	if err := json.Unmarshal(rotateRec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	newSecret, _ := rotated["clientSecret"].(string)
	if newSecret == "" || newSecret == originalSecret {
		t.Fatalf("expected rotate to issue a different, non-empty secret; original=%q new=%q", originalSecret, newSecret)
	}
	if rotated["id"] != id {
		t.Fatalf("expected rotate to keep the same credential id, got %+v", rotated)
	}

	// The old secret must no longer authenticate via the OAuth2 token
	// endpoint once rotated.
	oauthBody := "grant_type=client_credentials&client_id=" + id + "&client_secret=" + originalSecret
	oauthReq := httptest.NewRequest(http.MethodPost, "/api/oauth/token", bytes.NewBufferString(oauthBody))
	oauthReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	oauthRec := httptest.NewRecorder()
	mux.ServeHTTP(oauthRec, oauthReq)
	if oauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected the pre-rotation secret to be rejected, got %d: %s", oauthRec.Code, oauthRec.Body.String())
	}
}

// TestApplicationCredentialsAuditRedactsClientSecret guards against the
// plaintext client secret (returned exactly once by create/rotate) ever
// being persisted into the durable audit trail, which a much weaker
// audits:read permission can read back.
func TestApplicationCredentialsAuditRedactsClientSecret(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermApplicationCredentialsCreate)

	id, secret := createApplicationCredentialForTest(t, mux, token, "billing-service", envID, nil)

	auditToken := tokenFor(t, auth.PermAuditsRead)
	auditReq := httptest.NewRequest(http.MethodGet, "/api/audits?targetId="+id, nil)
	auditReq.Header.Set("Authorization", "Bearer "+auditToken)
	auditRec := httptest.NewRecorder()
	mux.ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("expected audits list to succeed, got %d: %s", auditRec.Code, auditRec.Body.String())
	}
	if bytes.Contains(auditRec.Body.Bytes(), []byte(secret)) {
		t.Fatalf("expected the plaintext client secret to never appear in the audit trail, got %s", auditRec.Body.String())
	}
	if !bytes.Contains(auditRec.Body.Bytes(), []byte("[redacted]")) {
		t.Fatalf("expected the audit entry's clientSecret field to be redacted, got %s", auditRec.Body.String())
	}
}
