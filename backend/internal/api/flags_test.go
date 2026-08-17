package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func TestFlagsFullLifecycle(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	createBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "value": "on", "environmentIds": []string{envID}})
	createReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected list to succeed, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var listed struct {
		Flags []store.Flag `json:"flags"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Flags) != 1 || listed.Flags[0].Key != "checkout" || !listed.Flags[0].Enabled {
		t.Fatalf("unexpected listed flags: %+v", listed.Flags)
	}

	// Overwrite via a second POST (upsert-by-key).
	updateBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": false, "value": "off", "environmentIds": []string{envID}})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	verifyReq.Header.Set("Authorization", "Bearer "+token)
	verifyRec := httptest.NewRecorder()
	mux.ServeHTTP(verifyRec, verifyReq)
	var updated struct {
		Flags []store.Flag `json:"flags"`
	}
	if err := json.Unmarshal(verifyRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if len(updated.Flags) != 1 || updated.Flags[0].Enabled {
		t.Fatalf("expected the later Set to win, got %+v", updated.Flags)
	}
}

func TestFlagsAreIsolatedPerEnvironmentThroughAPI(t *testing.T) {
	mux := newTestMux(t)
	prodID := seedEnvironmentForTest(t, "Production")
	stagingID := seedEnvironmentForTest(t, "Staging")
	token := tokenForWithEnvironments(t, []string{prodID, stagingID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	prodBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{prodID}})
	prodReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(prodBody))
	prodReq.Header.Set("Authorization", "Bearer "+token)
	prodRec := httptest.NewRecorder()
	mux.ServeHTTP(prodRec, prodReq)
	if prodRec.Code != http.StatusOK {
		t.Fatalf("expected prod create to succeed, got %d: %s", prodRec.Code, prodRec.Body.String())
	}

	stagingBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": false, "environmentIds": []string{stagingID}})
	stagingReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(stagingBody))
	stagingReq.Header.Set("Authorization", "Bearer "+token)
	stagingRec := httptest.NewRecorder()
	mux.ServeHTTP(stagingRec, stagingReq)
	if stagingRec.Code != http.StatusOK {
		t.Fatalf("expected staging create to succeed, got %d: %s", stagingRec.Code, stagingRec.Body.String())
	}

	prodListReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+prodID, nil)
	prodListReq.Header.Set("Authorization", "Bearer "+token)
	prodListRec := httptest.NewRecorder()
	mux.ServeHTTP(prodListRec, prodListReq)
	var prodListed struct {
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal(prodListRec.Body.Bytes(), &prodListed)
	if len(prodListed.Flags) != 1 || !prodListed.Flags[0].Enabled {
		t.Fatalf("expected prod's checkout to be enabled independently, got %+v", prodListed.Flags)
	}

	stagingListReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+stagingID, nil)
	stagingListReq.Header.Set("Authorization", "Bearer "+token)
	stagingListRec := httptest.NewRecorder()
	mux.ServeHTTP(stagingListRec, stagingListReq)
	var stagingListed struct {
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal(stagingListRec.Body.Bytes(), &stagingListed)
	if len(stagingListed.Flags) != 1 || stagingListed.Flags[0].Enabled {
		t.Fatalf("expected staging's checkout to be disabled independently, got %+v", stagingListed.Flags)
	}
}

func TestFlagsPostMultiEnvironmentCreateIsAtomic(t *testing.T) {
	mux := newTestMux(t)
	envA := seedEnvironmentForTest(t, "A")
	envB := seedEnvironmentForTest(t, "B")
	token := tokenForWithEnvironments(t, []string{envA, envB}, auth.PermFlagsRead, auth.PermFlagsWrite)

	body, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "value": "on", "environmentIds": []string{envA, envB}})
	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected multi-environment create to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Flags []store.Flag `json:"flags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Flags) != 2 {
		t.Fatalf("expected 2 flags in the response, got %+v", resp.Flags)
	}
	if resp.Flags[0].Version != resp.Flags[1].Version {
		t.Fatalf("expected both flags in the batch to share the same Version, got %+v", resp.Flags)
	}
}

func TestFlagsPostRejectsUnknownEnvironmentWithNoPartialWrites(t *testing.T) {
	mux := newTestMux(t)
	envA := seedEnvironmentForTest(t, "A")

	// Grant access to envA and a made-up ID so the environment-access check
	// passes and the failure comes from environment *existence* instead --
	// isolating the 400 (unknown environment) case from the 403 (access)
	// case already covered in middleware_test.go.
	token := tokenForWithEnvironments(t, []string{envA, "does-not-exist"}, auth.PermFlagsRead, auth.PermFlagsWrite)

	body, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{envA, "does-not-exist"}})
	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown environment ID, got %d: %s", rec.Code, rec.Body.String())
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envA, nil)
	verifyReq.Header.Set("Authorization", "Bearer "+token)
	verifyRec := httptest.NewRecorder()
	mux.ServeHTTP(verifyRec, verifyReq)
	var listed struct {
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal(verifyRec.Body.Bytes(), &listed)
	if len(listed.Flags) != 0 {
		t.Fatalf("expected no partial write to envA when the batch is rejected, got %+v", listed.Flags)
	}
}

func TestEnvironmentsDeleteRejectsWhenFlagsStillReferenceItThroughAPI(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedEnvironmentForTest(t, "Staging") // a second environment so ErrLastEnvironment doesn't also fire.
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite)

	flagBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{envID}})
	flagReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(flagBody))
	flagReq.Header.Set("Authorization", "Bearer "+token)
	flagRec := httptest.NewRecorder()
	mux.ServeHTTP(flagRec, flagReq)
	if flagRec.Code != http.StatusOK {
		t.Fatalf("expected flag create to succeed, got %d: %s", flagRec.Code, flagRec.Body.String())
	}

	deleteToken := tokenFor(t, auth.PermEnvironmentsDelete)
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/environments/"+envID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+deleteToken)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting an environment that still has flags, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestFlagsPutUpdatesEnabledAndValue(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite, auth.PermFlagsUpdate)

	createBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "value": "on", "environmentIds": []string{envID}})
	createReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	putBody, _ := json.Marshal(map[string]any{"enabled": false, "value": "off"})
	putReq := httptest.NewRequest(http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(putBody))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", putRec.Code, putRec.Body.String())
	}
	var updated store.Flag
	if err := json.Unmarshal(putRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Enabled || updated.Value != "off" {
		t.Fatalf("expected the PUT to win, got %+v", updated)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	var listed struct {
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &listed)
	if len(listed.Flags) != 1 || listed.Flags[0].Enabled || listed.Flags[0].Value != "off" {
		t.Fatalf("expected the updated flag to be reflected in the list, got %+v", listed.Flags)
	}
}

func TestFlagsPutReturnsNotFoundForUnknownFlag(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsUpdate)

	putBody, _ := json.Marshal(map[string]any{"enabled": true, "value": "on"})
	putReq := httptest.NewRequest(http.MethodPut, "/api/flags/does-not-exist?environmentId="+envID, bytes.NewReader(putBody))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 updating an unknown flag, got %d: %s", putRec.Code, putRec.Body.String())
	}
}

func TestFlagsPutRequiresPermFlagsUpdate(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead)

	putBody, _ := json.Marshal(map[string]any{"enabled": true, "value": "on"})
	putReq := httptest.NewRequest(http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(putBody))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 updating a flag with only flags:read, got %d: %s", putRec.Code, putRec.Body.String())
	}
}

func TestFlagsPutRequiresUpdateNotJustWrite(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	createBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{envID}})
	createReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	putBody, _ := json.Marshal(map[string]any{"enabled": false, "value": "off"})
	putReq := httptest.NewRequest(http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(putBody))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 updating a flag with only flags:write (no flags:update), got %d: %s", putRec.Code, putRec.Body.String())
	}
}

func TestFlagsDeleteRemovesFlag(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite, auth.PermFlagsDelete)

	createBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{envID}})
	createReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/flags/checkout?environmentId="+envID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete to succeed, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	var listed struct {
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &listed)
	if len(listed.Flags) != 0 {
		t.Fatalf("expected the flag to be gone after delete, got %+v", listed.Flags)
	}

	deleteAgainReq := httptest.NewRequest(http.MethodDelete, "/api/flags/checkout?environmentId="+envID, nil)
	deleteAgainReq.Header.Set("Authorization", "Bearer "+token)
	deleteAgainRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteAgainRec, deleteAgainReq)
	if deleteAgainRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 deleting an already-deleted flag, got %d: %s", deleteAgainRec.Code, deleteAgainRec.Body.String())
	}
}

func TestFlagsDeleteReturnsNotFoundForUnknownFlag(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsDelete)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/flags/does-not-exist?environmentId="+envID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 deleting an unknown flag, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestFlagsDeleteRequiresPermFlagsDeleteNotJustWrite(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	createBody, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{envID}})
	createReq := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/flags/checkout?environmentId="+envID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting a flag with only flags:write (no flags:delete), got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}
