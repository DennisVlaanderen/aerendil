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
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite, auth.PermFlagsUpdate)

	createBody := flagBody("checkout", true, "on", envID)
	createReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+envID, nil)
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

	// POST is create-only; changing an existing flag goes through PUT.
	updateBody := jsonBody(testFlag("", "", false, "off"))
	updateReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	verifyReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+envID, nil)
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
		t.Fatalf("expected the update to apply, got %+v", updated.Flags)
	}
}

func TestFlagsAreIsolatedPerEnvironmentThroughAPI(t *testing.T) {
	mux := newTestMux(t)
	prodID := seedEnvironmentForTest(t, "Production")
	stagingID := seedEnvironmentForTest(t, "Staging")
	token := tokenForWithEnvironments(t, []string{prodID, stagingID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	prodBody := flagBody("checkout", true, "", prodID)
	prodReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(prodBody))
	prodReq.Header.Set("Authorization", "Bearer "+token)
	prodRec := httptest.NewRecorder()
	mux.ServeHTTP(prodRec, prodReq)
	if prodRec.Code != http.StatusOK {
		t.Fatalf("expected prod create to succeed, got %d: %s", prodRec.Code, prodRec.Body.String())
	}

	stagingBody := flagBody("checkout", false, "", stagingID)
	stagingReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(stagingBody))
	stagingReq.Header.Set("Authorization", "Bearer "+token)
	stagingRec := httptest.NewRecorder()
	mux.ServeHTTP(stagingRec, stagingReq)
	if stagingRec.Code != http.StatusOK {
		t.Fatalf("expected staging create to succeed, got %d: %s", stagingRec.Code, stagingRec.Body.String())
	}

	prodListReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+prodID, nil)
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

	stagingListReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+stagingID, nil)
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

	body := flagBody("checkout", true, "on", envA, envB)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(body))
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
	if resp.Flags[0].Revision != resp.Flags[1].Revision {
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

	body := flagBody("checkout", true, "", envA, "does-not-exist")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown environment ID, got %d: %s", rec.Code, rec.Body.String())
	}

	verifyReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+envA, nil)
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

	flagBody := flagBody("checkout", true, "", envID)
	flagReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(flagBody))
	flagReq.Header.Set("Authorization", "Bearer "+token)
	flagRec := httptest.NewRecorder()
	mux.ServeHTTP(flagRec, flagReq)
	if flagRec.Code != http.StatusOK {
		t.Fatalf("expected flag create to succeed, got %d: %s", flagRec.Code, flagRec.Body.String())
	}

	deleteToken := tokenFor(t, auth.PermEnvironmentsDelete)
	deleteReq := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/environments/"+envID, nil)
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

	createBody := flagBody("checkout", true, "on", envID)
	createReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	putBody := jsonBody(testFlag("", "", false, "off"))
	putReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(putBody))
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
	if updated.Enabled || flagValue(updated) != "off" {
		t.Fatalf("expected the PUT to win, got %+v", updated)
	}

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	var listed struct {
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &listed)
	if len(listed.Flags) != 1 || listed.Flags[0].Enabled || flagValue(listed.Flags[0]) != "off" {
		t.Fatalf("expected the updated flag to be reflected in the list, got %+v", listed.Flags)
	}
}

// An omitted field must mean "unchanged", matching the contract already used
// for groupIds in usersPutHandler and active in applicationCredentialsPutHandler.
// Before the fix, a value-only PUT decoded enabled as false and silently turned
// the flag off.
func TestFlagsPutReturnsNotFoundForUnknownFlag(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsUpdate)

	putBody := jsonBody(testFlag("", "", true, "on"))
	putReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/flags/does-not-exist?environmentId="+envID, bytes.NewReader(putBody))
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

	putBody := jsonBody(testFlag("", "", true, "on"))
	putReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(putBody))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 updating a flag with only flags:read, got %d: %s", putRec.Code, putRec.Body.String())
	}
}

func TestFlagsGetByKeyReturnsFlag(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	createBody := flagBody("checkout", true, "on", envID)
	createReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags/checkout?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get to succeed, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var got store.Flag
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.Key != "checkout" || !got.Enabled || flagValue(got) != "on" || got.EnvironmentID != envID {
		t.Fatalf("unexpected flag returned: %+v", got)
	}
}

func TestFlagsGetByKeyReturnsNotFoundForUnknownFlag(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead)

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags/does-not-exist?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 getting an unknown flag, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestFlagsGetByKeyRequiresEnvironmentID(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead)

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags/checkout", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 getting a flag without environmentId, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestFlagsGetByKeyRequiresPermFlagsRead(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite)

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags/checkout?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 getting a flag without flags:read, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestFlagsGetByKeyRequiresEnvironmentAccess(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenFor(t, auth.PermFlagsRead)

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags/checkout?environmentId="+envID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 getting a flag for an environment the token isn't scoped to, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestFlagsPutRequiresUpdateNotJustWrite(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	createBody := flagBody("checkout", true, "", envID)
	createReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	putBody := jsonBody(testFlag("", "", false, "off"))
	putReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/flags/checkout?environmentId="+envID, bytes.NewReader(putBody))
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

	createBody := flagBody("checkout", true, "", envID)
	createReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	deleteReq := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/flags/checkout?environmentId="+envID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete to succeed, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/flags?environmentId="+envID, nil)
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

	deleteAgainReq := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/flags/checkout?environmentId="+envID, nil)
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

	deleteReq := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/flags/does-not-exist?environmentId="+envID, nil)
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

	createBody := flagBody("checkout", true, "", envID)
	createReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	deleteReq := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/flags/checkout?environmentId="+envID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting a flag with only flags:write (no flags:delete), got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

// POST /api/flags is create-only: flags:write must not be able to overwrite
// a flag that only flags:update may change (PUT /api/flags/{key}).
func TestFlagsPostRejectsExistingKeyWithoutOverwriting(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T, envIDs []string) string
	}{
		{"flags:write only", func(t *testing.T, envIDs []string) string {
			return tokenForWithEnvironments(t, envIDs, auth.PermFlagsWrite)
		}},
		{"admin", func(t *testing.T, _ []string) string { return adminToken(t) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := newTestMux(t)
			existingEnv := seedEnvironmentForTest(t, "Production")
			freshEnv := seedEnvironmentForTest(t, "Staging")
			if _, err := dataStore.Flags().Set(testFlag(existingEnv, "checkout", true, "on")); err != nil {
				t.Fatalf("seed flag: %v", err)
			}
			token := tt.token(t, []string{existingEnv, freshEnv})

			// The key exists in one of the two targets: the whole request
			// must be rejected, not just that environment.
			body := flagBody("checkout", false, "off", existingEnv, freshEnv)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusConflict {
				t.Fatalf("expected 409 for an existing key, got %d: %s", rec.Code, rec.Body.String())
			}

			existing, _ := dataStore.Flags().Get(existingEnv, "checkout")
			if !existing.Enabled || flagValue(existing) != "on" {
				t.Fatalf("expected the existing flag to be unchanged, got %+v", existing)
			}
			if fresh, found := dataStore.Flags().Get(freshEnv, "checkout"); found {
				t.Fatalf("expected no partial write to the other environment, got %+v", fresh)
			}
		})
	}
}

func TestFlagsForceDisable(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	otherEnvID := seedEnvironmentForTest(t, "Staging")
	if _, err := dataStore.Flags().Set(testFlag(envID, "kill-me", true, "active-value")); err != nil {
		t.Fatalf("seed flag: %v", err)
	}

	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsUpdate)

	// 1. Successful force-disable turns the flag off while keeping its value.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/kill-me/force-disable?environmentId="+envID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp store.Flag
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Enabled {
		t.Fatalf("expected flag to be disabled, got %+v", resp)
	}
	if flagValue(resp) != "active-value" {
		t.Fatalf("expected flag value to be preserved, got %q", flagValue(resp))
	}

	stored, exists := dataStore.Flags().Get(envID, "kill-me")
	if !exists || stored.Enabled {
		t.Fatalf("expected stored flag to be disabled, got %+v", stored)
	}

	// 2. Preserves full §6.1 definitions (non-string types, variations, rules, salt).
	richFlag := store.Flag{
		EnvironmentID: envID,
		Key:           "rich-boolean-flag",
		Type:          store.FlagTypeBoolean,
		Enabled:       true,
		Salt:          "custom-salt",
		Variations: []store.Variation{
			{ID: "off", Value: json.RawMessage(`false`)},
			{ID: "on", Value: json.RawMessage(`true`)},
		},
		OffVariation: "off",
		Rules: []store.Rule{
			{
				ID:    "beta-users",
				Serve: store.Serve{Variation: "on"},
			},
		},
		Fallthrough: store.Serve{Variation: "off"},
	}
	if _, err := dataStore.Flags().Set(richFlag); err != nil {
		t.Fatalf("seed rich flag: %v", err)
	}

	richReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/rich-boolean-flag/force-disable?environmentId="+envID, nil)
	richReq.Header.Set("Authorization", "Bearer "+token)
	richRec := httptest.NewRecorder()
	mux.ServeHTTP(richRec, richReq)
	if richRec.Code != http.StatusOK {
		t.Fatalf("expected 200 force-disabling boolean flag, got %d: %s", richRec.Code, richRec.Body.String())
	}
	storedRich, exists := dataStore.Flags().Get(envID, "rich-boolean-flag")
	if !exists || storedRich.Enabled {
		t.Fatalf("expected rich flag to be disabled, got %+v", storedRich)
	}
	if storedRich.Type != store.FlagTypeBoolean || storedRich.Salt != "custom-salt" || len(storedRich.Variations) != 2 || len(storedRich.Rules) != 1 {
		t.Fatalf("expected §6.1 type, salt, variations, and rules to be preserved, got %+v", storedRich)
	}

	// 3. Records distinct 'flag.force_disable' audit action with Before/After snapshots.
	audits := dataStore.Audits().List(store.AuditFilter{TargetType: "flag", TargetID: "kill-me"})
	var forceAudit *store.AuditEntry
	for i := range audits {
		if audits[i].Action == "flag.force_disable" && audits[i].Success {
			forceAudit = &audits[i]
			break
		}
	}
	if forceAudit == nil {
		t.Fatalf("expected successful 'flag.force_disable' audit entry, got %+v", audits)
	}
	if forceAudit.Before == "" || forceAudit.After == "" {
		t.Fatalf("expected Before and After snapshots on force_disable audit entry, got %+v", *forceAudit)
	}

	// 4. 400 Bad Request when environmentId is missing.
	reqNoEnv := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/kill-me/force-disable", nil)
	reqNoEnv.Header.Set("Authorization", "Bearer "+token)
	recNoEnv := httptest.NewRecorder()
	mux.ServeHTTP(recNoEnv, reqNoEnv)
	if recNoEnv.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing environmentId, got %d: %s", recNoEnv.Code, recNoEnv.Body.String())
	}

	// 5. 404 Not Found for unknown flag key.
	req404 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/non-existent/force-disable?environmentId="+envID, nil)
	req404.Header.Set("Authorization", "Bearer "+token)
	rec404 := httptest.NewRecorder()
	mux.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing flag, got %d: %s", rec404.Code, rec404.Body.String())
	}

	// 6. 403 Forbidden without environment access.
	reqNoEnvAccess := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/kill-me/force-disable?environmentId="+otherEnvID, nil)
	reqNoEnvAccess.Header.Set("Authorization", "Bearer "+token)
	recNoEnvAccess := httptest.NewRecorder()
	mux.ServeHTTP(recNoEnvAccess, reqNoEnvAccess)
	if recNoEnvAccess.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without environment access, got %d: %s", recNoEnvAccess.Code, recNoEnvAccess.Body.String())
	}

	// 7. 403 Forbidden without flags:update permission (even with flags:write).
	tokenWriteOnly := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)
	req403 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/kill-me/force-disable?environmentId="+envID, nil)
	req403.Header.Set("Authorization", "Bearer "+tokenWriteOnly)
	rec403 := httptest.NewRecorder()
	mux.ServeHTTP(rec403, req403)
	if rec403.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without flags:update permission, got %d: %s", rec403.Code, rec403.Body.String())
	}

	// 8. 401 Unauthorized without token.
	req401 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags/kill-me/force-disable?environmentId="+envID, nil)
	rec401 := httptest.NewRecorder()
	mux.ServeHTTP(rec401, req401)
	if rec401.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d: %s", rec401.Code, rec401.Body.String())
	}
}

// testFlag is a minimal valid §6.1 definition: one string variation, value,
// served everywhere.
func testFlag(environmentID, key string, enabled bool, value string) store.Flag {
	raw, _ := json.Marshal(value)
	return store.Flag{
		EnvironmentID: environmentID,
		Key:           key,
		Type:          store.FlagTypeString,
		Enabled:       enabled,
		Variations:    []store.Variation{{ID: "value", Value: raw}},
		OffVariation:  "value",
		Fallthrough:   store.Serve{Variation: "value"},
	}
}

func jsonBody(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// flagBody is testFlag as a POST /api/flags body for environmentIDs.
func flagBody(key string, enabled bool, value string, environmentIDs ...string) []byte {
	return jsonBody(struct {
		store.Flag
		EnvironmentIDs []string `json:"environmentIds"`
	}{testFlag("", key, enabled, value), environmentIDs})
}

// flagValue is the value a testFlag serves.
func flagValue(flag store.Flag) string {
	var value string
	if len(flag.Variations) > 0 {
		_ = json.Unmarshal(flag.Variations[0].Value, &value)
	}
	return value
}
