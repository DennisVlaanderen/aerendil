package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func TestUsersGetRequiresPermission(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without users:read, got %d", rec.Code)
	}
}

func TestUsersGetByIDRequiresPermission(t *testing.T) {
	mux := newTestMux(t)
	createToken := tokenFor(t, auth.PermUsersCreate)
	id := createUser(t, mux, createToken, "getbyid-noperm", "s3cret!!", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without users:read, got %d", rec.Code)
	}
}

func TestUsersGetByIDReturnsNotFoundForUnknownUser(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersGetByIDReturnsUserWithoutExposingPasswordHash(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersRead)
	id := createUser(t, mux, token, "getbyid-found", "s3cret!!", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users/"+id, nil)
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
	if raw["username"] != "getbyid-found" {
		t.Fatalf("expected username %q in response, got %+v", "getbyid-found", raw)
	}
	if _, present := raw["passwordHash"]; present {
		t.Fatal("expected password hash to never be present in the API response")
	}
}

func TestUsersPostCreatesUserWithoutExposingPasswordHash(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"username": "alice", "password": "s3cret!!", "groupIds": []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if raw["username"] != "alice" {
		t.Fatalf("expected username %q in response, got %+v", "alice", raw)
	}
	if _, present := raw["passwordHash"]; present {
		t.Fatal("expected password hash to never be present in the API response")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	getReq.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersRead))
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing users, got %d", getRec.Code)
	}
	if !bytes.Contains(getRec.Body.Bytes(), []byte("alice")) {
		t.Fatalf("expected created user to appear in the list, got %s", getRec.Body.String())
	}
}

// TestUsersResponsesNeverReturnNullGroupIDs guards against a regression where
// a user with no group memberships serialized as "groupIds":null instead of
// "groupIds":[] -- store.User.GroupIDs comes back nil (not an empty slice)
// after a Raft round-trip because of the "omitempty" JSON tag on the internal
// command envelope, and API clients (e.g. the dashboard/users UI) reasonably
// assume groupIds is always an array.
func TestUsersResponsesNeverReturnNullGroupIDs(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersRead)

	body, _ := json.Marshal(map[string]any{"username": "nogroups", "password": "s3cret!!"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"groupIds":null`)) {
		t.Fatalf("expected create response to never have null groupIds, got %s", rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if bytes.Contains(getRec.Body.Bytes(), []byte(`"groupIds":null`)) {
		t.Fatalf("expected list response to never have null groupIds, got %s", getRec.Body.String())
	}
}

func TestUsersPostRejectsDuplicateUsername(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate)

	body, _ := json.Marshal(map[string]any{"username": "bob", "password": "s3cret!!"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first create to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate username, got %d", rec2.Code)
	}
}

func TestUsersPostRejectsShortPassword(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"username": "shortpw", "password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a too-short password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersPostRejectsGrantingAdminGroupWithoutAdminPrivilege(t *testing.T) {
	mux := newTestMux(t)

	if _, ok := dataStore.Groups().Get(store.AdminGroupID); !ok {
		if _, err := dataStore.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
			t.Fatalf("seed admin group: %v", err)
		}
	}

	body, _ := json.Marshal(map[string]any{"username": "wannabe-admin", "password": "s3cret!!", "groupIds": []string{store.AdminGroupID}})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-admin trying to grant Admin group membership, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersPostAllowsAdminToGrantAdminGroup(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)

	body, _ := json.Marshal(map[string]any{"username": "new-admin", "password": "s3cret!!", "groupIds": []string{store.AdminGroupID}})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for admin granting Admin group membership, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	groupIDs, _ := raw["groupIds"].([]any)
	found := false
	for _, g := range groupIDs {
		if g == store.AdminGroupID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected response groupIds to include %q, got %+v", store.AdminGroupID, raw)
	}
}

func TestUsersPostRejectsOversizedBody(t *testing.T) {
	mux := newTestMux(t)

	oversizedUsername := strings.Repeat("a", maxRequestBodyBytes+1)
	body, _ := json.Marshal(map[string]any{"username": oversizedUsername, "password": "s3cret!!"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an oversized request body, got %d: %s", rec.Code, rec.Body.String())
	}
}

// createUser POSTs a new user via the API and returns its ID.
func createUser(t *testing.T, mux http.Handler, token string, username, password string, groupIDs []string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]any{"username": username, "password": password, "groupIds": groupIDs})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected user creation to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return created["id"].(string)
}

func TestUsersPutRequiresPermission(t *testing.T) {
	mux := newTestMux(t)
	createToken := tokenFor(t, auth.PermUsersCreate)
	id := createUser(t, mux, createToken, "editme", "s3cret!!", nil)

	body, _ := json.Marshal(map[string]any{"username": "editme", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without users:update, got %d", rec.Code)
	}
}

func TestUsersPutReturnsNotFoundForUnknownUser(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"username": "ghost", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/does-not-exist", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersUpdate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersPutUpdatesUsernameGroupsAndActive(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	if _, err := dataStore.Groups().Set(store.Group{ID: "editors", Name: "Editors"}); err != nil {
		t.Fatalf("create editors group: %v", err)
	}
	id := createUser(t, mux, token, "before", "s3cret!!", nil)

	body, _ := json.Marshal(map[string]any{"username": "after", "groupIds": []string{"editors"}, "active": false})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
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
	if raw["username"] != "after" {
		t.Fatalf("expected username to be updated, got %+v", raw)
	}
	if raw["active"] != false {
		t.Fatalf("expected active to be updated to false, got %+v", raw)
	}

	updated, ok := dataStore.Users().Get(id)
	if !ok {
		t.Fatal("expected user to still exist after update")
	}
	if len(updated.GroupIDs) != 1 || updated.GroupIDs[0] != "editors" {
		t.Fatalf("expected groupIds to be updated, got %+v", updated.GroupIDs)
	}
}

func TestUsersPutRejectsDuplicateUsername(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	createUser(t, mux, token, "taken", "s3cret!!", nil)
	id := createUser(t, mux, token, "other", "s3cret!!", nil)

	body, _ := json.Marshal(map[string]any{"username": "taken", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a username taken by another user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersPutAllowsKeepingSameUsername(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	id := createUser(t, mux, token, "unchanged", "s3cret!!", nil)

	body, _ := json.Marshal(map[string]any{"username": "unchanged", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when keeping the same username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersPutKeepsPasswordWhenOmitted(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	id := createUser(t, mux, token, "keeppw", "original!", nil)

	body, _ := json.Marshal(map[string]any{"username": "keeppw", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	loginBody, _ := json.Marshal(map[string]any{"username": "keeppw", "password": "original!"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected original password to still work after an edit that omitted it, got %d: %s", loginRec.Code, loginRec.Body.String())
	}
}

func TestUsersPutChangesPasswordWhenProvided(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	id := createUser(t, mux, token, "changepw", "original!", nil)

	body, _ := json.Marshal(map[string]any{"username": "changepw", "password": "changed!!", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	loginBody, _ := json.Marshal(map[string]any{"username": "changepw", "password": "changed!!"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected new password to work after edit, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	oldLoginBody, _ := json.Marshal(map[string]any{"username": "changepw", "password": "original!"})
	oldLoginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(oldLoginBody))
	oldLoginRec := httptest.NewRecorder()
	mux.ServeHTTP(oldLoginRec, oldLoginReq)
	if oldLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password to no longer work after edit, got %d", oldLoginRec.Code)
	}
}

func TestUsersPutRejectsGrantingAdminGroupWithoutAdminPrivilege(t *testing.T) {
	mux := newTestMux(t)
	if _, ok := dataStore.Groups().Get(store.AdminGroupID); !ok {
		if _, err := dataStore.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
			t.Fatalf("seed admin group: %v", err)
		}
	}
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	id := createUser(t, mux, token, "promoteme", "s3cret!!", nil)

	body, _ := json.Marshal(map[string]any{"username": "promoteme", "groupIds": []string{store.AdminGroupID}, "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-admin trying to grant Admin group membership via edit, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersDeleteRequiresPermission(t *testing.T) {
	mux := newTestMux(t)
	createToken := tokenFor(t, auth.PermUsersCreate)
	id := createUser(t, mux, createToken, "deleteme1", "s3cret!!", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without users:delete, got %d", rec.Code)
	}
}

func TestUsersDeleteReturnsNotFoundForUnknownUser(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersDelete))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsersDeleteRemovesUser(t *testing.T) {
	mux := newTestMux(t)
	createToken := tokenFor(t, auth.PermUsersCreate)
	id := createUser(t, mux, createToken, "deleteme2", "s3cret!!", nil)

	// A successful delete requires admin (see the hardcode in
	// usersDeleteHandler), not just the users:delete permission --
	// adminToken bypasses permission checks entirely.
	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := dataStore.Users().Get(id); ok {
		t.Fatal("expected user to be gone after delete")
	}
}

// TestUsersDeleteRejectsNonAdminEvenWithDeletePermission documents the
// deliberate discrepancy from Groups: deleting a user is hardcoded to
// admins only (see usersDeleteHandler), so holding users:delete alone
// isn't enough -- unlike groups:delete, which has no such extra check.
func TestUsersDeleteRejectsNonAdminEvenWithDeletePermission(t *testing.T) {
	mux := newTestMux(t)
	createToken := tokenFor(t, auth.PermUsersCreate)
	id := createUser(t, mux, createToken, "deleteme3", "s3cret!!", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersDelete))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-admin with users:delete, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := dataStore.Users().Get(id); !ok {
		t.Fatal("expected the user to still exist after a rejected delete")
	}
}

// idFromToken resolves the user ID a bearer token (as produced by
// tokenFor/adminToken) was issued for, by round-tripping it through
// /api/auth/me -- the tests below need to act as, and later try to delete
// or demote, that exact account.
func idFromToken(t *testing.T, mux http.Handler, token string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/auth/me, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode /api/auth/me response: %v", err)
	}
	user, _ := raw["user"].(map[string]any)
	id, _ := user["id"].(string)
	if id == "" {
		t.Fatalf("expected /api/auth/me to include a user id, got %+v", raw)
	}
	return id
}

// TestUsersDeleteRejectsSoleRemainingAdmin guards the invariant that keeps
// the cluster always manageable: the Admin group itself is undeletable
// (see TestAdminGroupCannotBeDeleted), but that protection is worthless if
// the last account that's actually a member of it can still be deleted.
func TestUsersDeleteRejectsSoleRemainingAdmin(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)
	id := idFromToken(t, mux, token)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting the sole remaining admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := dataStore.Users().Get(id); !ok {
		t.Fatal("expected the sole remaining admin to still exist after a rejected delete")
	}
}

// TestUsersDeleteRejectsNonAdminDeletingAnotherAdmin guards a distinct gap
// from the sole-remaining-admin protection above: even when another admin
// exists (so store.ErrLastAdmin doesn't fire), a caller who isn't admin
// themselves -- regardless of permissions -- must never be able to delete
// an admin account.
func TestUsersDeleteRejectsNonAdminDeletingAnotherAdmin(t *testing.T) {
	mux := newTestMux(t)
	adminID := idFromToken(t, mux, adminToken(t))
	nonAdminToken := tokenFor(t, auth.PermUsersDelete)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+adminID, nil)
	req.Header.Set("Authorization", "Bearer "+nonAdminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-admin users:delete caller deleting an admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := dataStore.Users().Get(adminID); !ok {
		t.Fatal("expected the admin account to still exist after a rejected delete")
	}
}

// TestUsersPutRejectsNonAdminRemovingAnotherAdminsAdminGroup guards the
// removal side of Admin group membership: a non-admin users:update caller
// must not be able to strip another admin's Admin group membership just
// because the proposed new group list happens to omit it.
func TestUsersPutRejectsNonAdminRemovingAnotherAdminsAdminGroup(t *testing.T) {
	mux := newTestMux(t)
	adminID := idFromToken(t, mux, adminToken(t))
	// A second admin so this isn't blocked by store.ErrLastAdmin instead.
	adminToken(t)
	nonAdminToken := tokenFor(t, auth.PermUsersUpdate)

	body, _ := json.Marshal(map[string]any{"username": "user-" + adminID, "groupIds": []string{}, "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+adminID, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+nonAdminToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-admin users:update caller removing another admin's Admin group membership, got %d: %s", rec.Code, rec.Body.String())
	}
	got, ok := dataStore.Users().Get(adminID)
	if !ok || len(got.GroupIDs) == 0 {
		t.Fatalf("expected the target admin to keep Admin group membership, got %+v (ok=%v)", got, ok)
	}
}

// TestUsersPutOmittingGroupIDsPreservesExistingMembership guards the fix
// for a silent-data-loss gap: a caller who can't see the full group list
// (e.g. missing groups:read, so the UI can't render group checkboxes at
// all) must still be able to edit a user's other fields without an omitted
// groupIds field being treated as "clear all groups".
func TestUsersPutOmittingGroupIDsPreservesExistingMembership(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	if _, err := dataStore.Groups().Set(store.Group{ID: "editors", Name: "Editors"}); err != nil {
		t.Fatalf("create editors group: %v", err)
	}
	id := createUser(t, mux, token, "keepgroups", "s3cret!!", []string{"editors"})

	// No "groupIds" key at all in this body -- distinct from an explicit [].
	body, _ := json.Marshal(map[string]any{"username": "keepgroups", "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got, ok := dataStore.Users().Get(id)
	if !ok || len(got.GroupIDs) != 1 || got.GroupIDs[0] != "editors" {
		t.Fatalf("expected groupIds to be left unchanged when omitted, got %+v (ok=%v)", got.GroupIDs, ok)
	}
}

// TestUsersPostRejectsOversizedPassword guards the fix that maps a password
// over bcrypt's 72-byte input limit to a 400, rather than letting
// bcrypt.GenerateFromPassword fail and fall through to a generic 500.
func TestUsersPostRejectsOversizedPassword(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"username": "longpw", "password": strings.Repeat("a", 73)})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermUsersCreate))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an oversized password, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUsersUsernameIsCaseInsensitive guards against "admin" and "Admin"
// coexisting as distinct accounts: usernames are normalized to lowercase on
// create, so a differently-cased username collides with an existing one.
func TestUsersUsernameIsCaseInsensitive(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermUsersCreate)

	body, _ := json.Marshal(map[string]any{"username": "CaseTest", "password": "s3cret!!"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first create to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	body2, _ := json.Marshal(map[string]any{"username": "casetest", "password": "s3cret!!"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body2))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a username differing only in case, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// TestUsersDeleteAllowsAdminWhenAnotherAdminRemains checks the flip side:
// the protection is specifically about the *last* admin, not admins in
// general, so deleting one of several admins must still work.
func TestUsersDeleteAllowsAdminWhenAnotherAdminRemains(t *testing.T) {
	mux := newTestMux(t)
	firstToken := adminToken(t)
	firstID := idFromToken(t, mux, firstToken)
	secondToken := adminToken(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+firstID, nil)
	req.Header.Set("Authorization", "Bearer "+secondToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting one of two admins, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := dataStore.Users().Get(firstID); ok {
		t.Fatal("expected the deleted admin to be gone")
	}
}

// TestUsersPutRejectsDeactivatingSoleRemainingAdmin covers the same
// invariant via the other route that can strand the cluster with no
// admin: editing the sole admin inactive instead of deleting them outright.
func TestUsersPutRejectsDeactivatingSoleRemainingAdmin(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)
	id := idFromToken(t, mux, token)

	body, _ := json.Marshal(map[string]any{"username": "user-" + id, "groupIds": []string{store.AdminGroupID}, "active": false})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deactivating the sole remaining admin, got %d: %s", rec.Code, rec.Body.String())
	}

	got, ok := dataStore.Users().Get(id)
	if !ok || !got.Active {
		t.Fatalf("expected the sole remaining admin to stay active, got %+v (ok=%v)", got, ok)
	}
}

// TestUsersPutRejectsRemovingSoleRemainingAdminFromAdminGroup covers the
// third way to strand the cluster: keeping the account active but editing
// it out of the Admin group entirely.
func TestUsersPutRejectsRemovingSoleRemainingAdminFromAdminGroup(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)
	id := idFromToken(t, mux, token)

	body, _ := json.Marshal(map[string]any{"username": "user-" + id, "groupIds": []string{}, "active": true})
	req := httptest.NewRequest(http.MethodPut, "/api/users/"+id, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 removing the sole remaining admin's Admin group membership, got %d: %s", rec.Code, rec.Body.String())
	}

	got, ok := dataStore.Users().Get(id)
	if !ok || len(got.GroupIDs) == 0 {
		t.Fatalf("expected the sole remaining admin to keep Admin group membership, got %+v (ok=%v)", got, ok)
	}
}
