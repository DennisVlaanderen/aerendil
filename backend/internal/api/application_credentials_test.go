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
	token := tokenFor(t, auth.PermApplicationCredentialsCreate, auth.PermApplicationCredentialsRead,
		auth.PermApplicationCredentialsUpdate, auth.PermApplicationCredentialsDelete, auth.PermEnvironmentsCreate)
	envID := seedEnvironmentForTest(t, "Production")

	id, secret := createApplicationCredentialForTest(t, mux, token, "billing-service", envID, []string{auth.PermFlagsRead})
	if secret == "" {
		t.Fatal("expected a non-empty client secret")
	}

	updateBody, _ := json.Marshal(map[string]any{
		"name":          "billing-service-renamed",
		"environmentId": envID,
		"scopes":        []string{auth.PermFlagsRead, auth.PermFlagsWrite},
		"active":        true,
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

	listReq := httptest.NewRequest(http.MethodGet, "/api/application-credentials", nil)
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

func TestApplicationCredentialsPostRejectsUnknownScope(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermApplicationCredentialsCreate)
	envID := seedEnvironmentForTest(t, "Production")

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
	token := tokenFor(t, auth.PermApplicationCredentialsCreate)

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

func TestApplicationCredentialsPutAndDeleteReturnNotFoundForUnknownID(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermApplicationCredentialsUpdate, auth.PermApplicationCredentialsDelete)
	envID := seedEnvironmentForTest(t, "Production")

	updateBody, _ := json.Marshal(map[string]any{"name": "x", "environmentId": envID, "scopes": []string{}})
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
	token := tokenFor(t, auth.PermApplicationCredentialsCreate, auth.PermApplicationCredentialsUpdate)
	envID := seedEnvironmentForTest(t, "Production")

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
