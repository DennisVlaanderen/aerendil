package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func TestGroupsPostRejectsUnknownPermission(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"name": "Editors", "permissions": []string{"not-a-real-permission"}})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown permission string, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroupsPostRejectsUnknownEnvironment(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"name": "Editors", "environmentIds": []string{"does-not-exist"}})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown environment ID, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroupsPostAcceptsKnownEnvironment(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")

	body, _ := json.Marshal(map[string]any{"name": "Editors", "environmentIds": []string{envID}})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	environmentIDs, ok := raw["environmentIds"].([]any)
	if !ok || len(environmentIDs) != 1 || environmentIDs[0] != envID {
		t.Fatalf("expected environmentIds to include %q, got %+v", envID, raw["environmentIds"])
	}
}

func TestGroupsPostForcesNonSystem(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"name": "Editors", "permissions": []string{auth.PermFlagsRead}, "system": true})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if raw["system"] != false {
		t.Fatalf("expected a client-supplied system:true to be ignored, got %+v", raw)
	}
}

func TestGroupsFullCRUDForNonSystemGroup(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermGroupsCreate, auth.PermGroupsUpdate, auth.PermGroupsDelete)

	createBody, _ := json.Marshal(map[string]any{"name": "Editors", "permissions": []string{auth.PermFlagsRead}})
	createReq := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := created["id"].(string)

	updateBody, _ := json.Marshal(map[string]any{"name": "Editors v2", "permissions": []string{auth.PermFlagsRead, auth.PermFlagsWrite}})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/groups/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/groups/"+id, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete to succeed, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	listReq.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsRead))
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if bytes.Contains(listRec.Body.Bytes(), []byte("Editors v2")) {
		t.Fatalf("expected deleted group to be gone from the list, got %s", listRec.Body.String())
	}
}

// TestGroupsResponsesNeverReturnNullPermissions guards against a regression
// where a group with no permissions serialized as "permissions":null instead
// of "permissions":[] -- store.Group.Permissions comes back nil (not an
// empty slice) after a Raft round-trip because of the "omitempty" JSON tag,
// and the dashboard/groups UI calls .includes() on it unconditionally.
func TestGroupsResponsesNeverReturnNullPermissions(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermGroupsCreate, auth.PermGroupsRead)

	body, _ := json.Marshal(map[string]any{"name": "NoPerms", "permissions": []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"permissions":null`)) {
		t.Fatalf("expected create response to never have null permissions, got %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"environmentIds":null`)) {
		t.Fatalf("expected create response to never have null environmentIds, got %s", rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if bytes.Contains(getRec.Body.Bytes(), []byte(`"permissions":null`)) {
		t.Fatalf("expected list response to never have null permissions, got %s", getRec.Body.String())
	}
	if bytes.Contains(getRec.Body.Bytes(), []byte(`"environmentIds":null`)) {
		t.Fatalf("expected list response to never have null environmentIds, got %s", getRec.Body.String())
	}
}

// TestAdminGroupProtectionAppliesEvenWithoutAPILayerPreCheck guards the
// consolidated protection: groupsPutHandler/groupsDeleteHandler no longer
// pre-check the Admin group ID themselves, relying entirely on
// store.GroupRepository returning store.ErrProtectedSystemGroup. This
// exercises that store-layer path directly to make sure removing the
// API-layer duplication didn't quietly drop the protection.
func TestAdminGroupProtectionAppliesEvenWithoutAPILayerPreCheck(t *testing.T) {
	mux := newTestMux(t)
	if _, ok := dataStore.Groups().Get(store.AdminGroupID); !ok {
		if _, err := dataStore.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
			t.Fatalf("seed admin group: %v", err)
		}
	}

	if _, err := dataStore.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Renamed"}); err == nil {
		t.Fatal("expected Set on the Admin group to fail")
	} else if !errors.Is(err, store.ErrProtectedSystemGroup) {
		t.Fatalf("expected ErrProtectedSystemGroup, got %v", err)
	}
	if err := dataStore.Groups().Delete(store.AdminGroupID); !errors.Is(err, store.ErrProtectedSystemGroup) {
		t.Fatalf("expected ErrProtectedSystemGroup, got %v", err)
	}

	_ = mux
}

// seedGroupForTest creates a non-system group directly through the store,
// mirroring seedEnvironmentForTest, so GET-by-ID tests don't need to round-
// trip through the create endpoint.
func seedGroupForTest(t *testing.T, name string, perms []string) string {
	t.Helper()

	g, err := dataStore.Groups().Set(store.Group{ID: store.NewID(), Name: name, Permissions: perms})
	if err != nil {
		t.Fatalf("seed group %q: %v", name, err)
	}
	return g.ID
}

func TestGroupsGetByIDRequiresPermission(t *testing.T) {
	mux := newTestMux(t)
	id := seedGroupForTest(t, "GetByIDNoPerm", nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/groups/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without groups:read, got %d", rec.Code)
	}
}

func TestGroupsGetByIDReturnsNotFoundForUnknownGroup(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/groups/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown group, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroupsGetByIDReturnsGroup(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermGroupsRead)
	id := seedGroupForTest(t, "GetByIDFound", []string{auth.PermFlagsRead})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/groups/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if raw["id"] != id {
		t.Fatalf("expected id %q in response, got %+v", id, raw)
	}
	if raw["name"] != "GetByIDFound" {
		t.Fatalf("expected name %q in response, got %+v", "GetByIDFound", raw)
	}
}

func TestGroupsDeleteReturnsNotFoundForUnknownGroup(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermGroupsDelete))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleting an unknown group, got %d: %s", rec.Code, rec.Body.String())
	}
}
