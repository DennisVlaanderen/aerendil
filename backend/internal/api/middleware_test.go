package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

// newTestMux boots a real (single-node, temp-dir-backed) store and wires it
// up through RegisterRoutes, then waits for the node to become raft leader
// by retrying a harmless Set until it succeeds.
func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()

	dataDir, err := os.MkdirTemp("", "aerendil-middleware-test-*")
	if err != nil {
		t.Fatalf("create temp data dir: %v", err)
	}
	// store.Close doesn't close the underlying BoltDB file handle, so on
	// Windows a t.TempDir()-style cleanup can race an open file lock. Best
	// effort removal here avoids failing this test over that pre-existing,
	// unrelated cleanup gap.
	t.Cleanup(func() {
		_ = os.RemoveAll(dataDir)
	})

	s, err := store.Open(store.Config{
		NodeID:    "middleware-test",
		BindAddr:  "127.0.0.1:0",
		DataDir:   dataDir,
		Bootstrap: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Groups().Set is the warmup probe (not Flags().Set) because Flag.Set now
	// validates its EnvironmentID against an existing Environment -- there is
	// none yet at this point, so it would never succeed. A throwaway,
	// never-referenced Group has no such referential-integrity or ordering
	// side effects on other tests (unlike Environment, whose Order field
	// several environments_test.go assertions depend on being 0-based).
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := s.Groups().Set(store.Group{ID: store.NewID(), Name: "warmup"}); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for raft node to become leader: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, s)
	return mux
}

// tokenFor creates a user holding perms (via a throwaway non-system group)
// and returns a bearer token for them. Call newTestMux first so the
// package-level dataStore/authService are wired up.
func tokenFor(t *testing.T, perms ...string) string {
	t.Helper()

	groupID := store.NewID()
	if _, err := dataStore.Groups().Set(store.Group{ID: groupID, Name: "test-group", Permissions: perms}); err != nil {
		t.Fatalf("create test group: %v", err)
	}
	return tokenForGroups(t, groupID)
}

// tokenForWithEnvironments is tokenFor's environment-aware counterpart --
// creates a group holding both perms and access to environmentIDs, and
// returns a bearer token for a new user in that group.
func tokenForWithEnvironments(t *testing.T, environmentIDs []string, perms ...string) string {
	t.Helper()

	groupID := store.NewID()
	if _, err := dataStore.Groups().Set(store.Group{ID: groupID, Name: "test-group", Permissions: perms, EnvironmentIDs: environmentIDs}); err != nil {
		t.Fatalf("create test group: %v", err)
	}
	return tokenForGroups(t, groupID)
}

// seedEnvironmentForTest creates an Environment via the package-level
// dataStore and returns its ID -- for tests that need one to scope flags
// to. Call newTestMux first so dataStore is wired up.
func seedEnvironmentForTest(t *testing.T, name string) string {
	t.Helper()

	env, err := dataStore.Environments().Set(store.Environment{ID: store.NewID(), Name: name})
	if err != nil {
		t.Fatalf("seed environment %q: %v", name, err)
	}
	return env.ID
}

// adminToken creates a user in the (seeded-if-needed) Admin group and
// returns a bearer token for them.
func adminToken(t *testing.T) string {
	t.Helper()

	if _, ok := dataStore.Groups().Get(store.AdminGroupID); !ok {
		if _, err := dataStore.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
			t.Fatalf("seed admin group: %v", err)
		}
	}
	return tokenForGroups(t, store.AdminGroupID)
}

func tokenForGroups(t *testing.T, groupIDs ...string) string {
	t.Helper()

	user, err := dataStore.Users().Set(store.User{
		ID:       store.NewID(),
		Username: "user-" + store.NewID(),
		Active:   true,
		GroupIDs: groupIDs,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	token, err := authService.GenerateToken(&auth.User{ID: user.ID, Username: user.Username})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

func TestFlagsGetRequiresAuthentication(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d", rec.Code)
	}
}

func TestFlagsGetRequiresReadPermission(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a user with no permissions, got %d", rec.Code)
	}
}

func TestFlagsGetSucceedsWithReadPermission(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")

	req := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a user with flags:read and environment access, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFlagsGetRequiresEnvironmentIDQueryParam(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermFlagsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with no environmentId query param, got %d", rec.Code)
	}
}

func TestFlagsGetRequiresEnvironmentAccess(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")

	req := httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil)
	// flags:read but no grant for this specific environment.
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermFlagsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for flags:read without access to this environment, got %d", rec.Code)
	}
}

func TestFlagsPostRequiresWritePermission(t *testing.T) {
	mux := newTestMux(t)

	body, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true})

	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermFlagsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a user with only flags:read, got %d", rec.Code)
	}
}

func TestFlagsPostSucceedsWithWritePermission(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")

	body, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "value": "on", "environmentIds": []string{envID}})

	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a user with flags:write and environment access, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Flags []store.Flag `json:"flags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Flags) != 1 || resp.Flags[0].Key != "checkout" || !resp.Flags[0].Enabled || resp.Flags[0].Value != "on" {
		t.Fatalf("unexpected flags in response: %+v", resp.Flags)
	}
}

func TestFlagsPostRequiresEnvironmentAccessForEveryRequestedEnvironment(t *testing.T) {
	mux := newTestMux(t)
	grantedID := seedEnvironmentForTest(t, "Production")
	ungrantedID := seedEnvironmentForTest(t, "Staging")

	body, _ := json.Marshal(map[string]any{"key": "checkout", "enabled": true, "environmentIds": []string{grantedID, ungrantedID}})

	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	// Only granted access to one of the two requested environments.
	req.Header.Set("Authorization", "Bearer "+tokenForWithEnvironments(t, []string{grantedID}, auth.PermFlagsWrite))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when access to any requested environment is missing, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, ok := dataStore.Flags().Get(grantedID, "checkout"); ok {
		t.Fatal("expected no flag to be written to the granted environment when the request as a whole is rejected")
	}
}

func TestAdminGroupBypassesAllPermissionChecks(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)
	envID := seedEnvironmentForTest(t, "Production")

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/flags?environmentId="+envID, nil),
		httptest.NewRequest(http.MethodGet, "/api/users", nil),
		httptest.NewRequest(http.MethodGet, "/api/groups", nil),
	} {
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for admin on %s, got %d: %s", req.URL.Path, rec.Code, rec.Body.String())
		}
	}
}

func TestAdminGroupCannotBeEdited(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)

	body, _ := json.Marshal(map[string]any{"name": "Renamed", "permissions": []string{}})
	req := httptest.NewRequest(http.MethodPut, "/api/groups/"+store.AdminGroupID, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 editing the Admin group even as admin, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminGroupCannotBeDeleted(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/"+store.AdminGroupID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting the Admin group even as admin, got %d: %s", rec.Code, rec.Body.String())
	}
}
