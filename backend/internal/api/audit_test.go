package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func getAudits(t *testing.T, mux *http.ServeMux, token string, query string) []store.AuditEntry {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/audits"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/audits, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Audits []store.AuditEntry `json:"audits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode audits response: %v", err)
	}
	return payload.Audits
}

func TestAuditsGetRequiresPermission(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/audits", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a user without audits:read, got %d", rec.Code)
	}
}

func TestAuditRecordsSuccessfulUserUpdateWithoutLeakingPasswordHash(t *testing.T) {
	mux := newTestMux(t)
	writeToken := tokenFor(t, auth.PermUsersCreate, auth.PermUsersUpdate)
	readToken := tokenFor(t, auth.PermAuditsRead)

	createBody, _ := json.Marshal(map[string]any{"username": "alice", "password": "hunter22", "groupIds": []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating user, got %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	updateBody, _ := json.Marshal(map[string]any{"username": "alice2", "active": true, "groupIds": []string{}})
	req = httptest.NewRequest(http.MethodPut, "/api/users/"+created.ID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating user, got %d: %s", rec.Code, rec.Body.String())
	}

	entries := getAudits(t, mux, readToken, "?targetType=user&targetId="+created.ID)
	var updateEntry *store.AuditEntry
	for i := range entries {
		if entries[i].Action == "user.update" {
			updateEntry = &entries[i]
		}
	}
	if updateEntry == nil {
		t.Fatalf("expected a user.update audit entry, got %+v", entries)
	}
	if !updateEntry.Success || updateEntry.StatusCode != http.StatusOK {
		t.Fatalf("expected a successful update entry, got %+v", updateEntry)
	}
	if updateEntry.Before == "" || updateEntry.After == "" {
		t.Fatalf("expected both Before and After to be populated, got %+v", updateEntry)
	}
	if strings.Contains(updateEntry.Before, "password") || strings.Contains(updateEntry.After, "password") {
		t.Fatalf("expected no password hash to leak into the audit trail, got %+v", updateEntry)
	}
	if !strings.Contains(updateEntry.Before, "alice") || !strings.Contains(updateEntry.After, "alice2") {
		t.Fatalf("expected Before/After to reflect the username change, got %+v", updateEntry)
	}
}

func TestAuditRecordsRejectedMutation(t *testing.T) {
	mux := newTestMux(t)
	writeToken := tokenFor(t, auth.PermUsersCreate)
	readToken := tokenFor(t, auth.PermAuditsRead)

	body, _ := json.Marshal(map[string]any{"username": "bob", "password": "hunter22", "groupIds": []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating first user, got %d: %s", rec.Code, rec.Body.String())
	}

	// Duplicate username -> rejected.
	dupBody, _ := json.Marshal(map[string]any{"username": "bob", "password": "hunter22", "groupIds": []string{}})
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(dupBody))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate username, got %d: %s", rec.Code, rec.Body.String())
	}

	entries := getAudits(t, mux, readToken, "")
	var rejected *store.AuditEntry
	for i := range entries {
		if entries[i].Action == "user.create" && !entries[i].Success {
			rejected = &entries[i]
		}
	}
	if rejected == nil {
		t.Fatalf("expected a rejected user.create audit entry, got %+v", entries)
	}
	if rejected.StatusCode != http.StatusConflict {
		t.Fatalf("expected StatusCode 409 on the rejected entry, got %+v", rejected)
	}
	if rejected.Error == "" {
		t.Fatalf("expected a populated Error on the rejected entry, got %+v", rejected)
	}
	if rejected.After != "" {
		t.Fatalf("expected no After state on a rejected create, got %+v", rejected)
	}
}

func TestAuditRecordsFlagUpsertBeforeState(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	writeToken := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite)
	readToken := tokenFor(t, auth.PermAuditsRead)

	key := fmt.Sprintf("flag-%s", store.NewID())

	firstBody, _ := json.Marshal(map[string]any{"key": key, "enabled": true, "value": "on", "environmentIds": []string{envID}})
	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(firstBody))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on first flag set, got %d: %s", rec.Code, rec.Body.String())
	}

	secondBody, _ := json.Marshal(map[string]any{"key": key, "enabled": false, "value": "off", "environmentIds": []string{envID}})
	req = httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(secondBody))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on second flag set, got %d: %s", rec.Code, rec.Body.String())
	}

	entries := getAudits(t, mux, readToken, "?targetType=flag&targetId="+key)
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries for %q, got %d: %+v", key, len(entries), entries)
	}

	// Newest first: entries[0] is the second set, whose Before should
	// reflect the first flag's prior (enabled=true) state.
	second := entries[0]
	if second.Before == "" || !strings.Contains(second.Before, `"enabled":true`) {
		t.Fatalf("expected second entry's Before to reflect the prior enabled=true state, got %+v", second)
	}

	first := entries[1]
	if first.Before != "" {
		t.Fatalf("expected first entry to have no prior state, got %+v", first)
	}
}

func TestAuditsListFiltersByActorID(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	writeToken := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite)
	readToken := tokenFor(t, auth.PermAuditsRead)

	key := fmt.Sprintf("actor-filter-flag-%s", store.NewID())
	body, _ := json.Marshal(map[string]any{"key": key, "enabled": true, "environmentIds": []string{envID}})
	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 setting flag, got %d: %s", rec.Code, rec.Body.String())
	}

	all := getAudits(t, mux, readToken, "?targetType=flag&targetId="+key)
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 audit entry for %q, got %+v", key, all)
	}
	actorID := all[0].ActorID
	if actorID == "" {
		t.Fatalf("expected the entry to have a non-empty ActorID, got %+v", all[0])
	}

	byActor := getAudits(t, mux, readToken, "?actorId="+actorID)
	found := false
	for _, e := range byActor {
		if e.TargetType == "flag" && e.TargetID == key {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected filtering by actorId=%s to include the flag.set entry, got %+v", actorID, byActor)
	}

	byNonexistentActor := getAudits(t, mux, readToken, "?actorId=does-not-exist")
	if len(byNonexistentActor) != 0 {
		t.Fatalf("expected no entries for a nonexistent actorId, got %+v", byNonexistentActor)
	}
}
