package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func getAudits(t *testing.T, mux *http.ServeMux, token string, query string) []auditEntryView {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/audits"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/audits, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Audits []auditEntryView `json:"audits"`
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
	var updateEntry *auditEntryView
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
	if len(updateEntry.Before) == 0 || len(updateEntry.After) == 0 {
		t.Fatalf("expected both Before and After to be populated, got %+v", updateEntry)
	}
	if strings.Contains(string(updateEntry.Before), "password") || strings.Contains(string(updateEntry.After), "password") {
		t.Fatalf("expected no password hash to leak into the audit trail, got %+v", updateEntry)
	}
	if !strings.Contains(string(updateEntry.Before), "alice") || !strings.Contains(string(updateEntry.After), "alice2") {
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
	var rejected *auditEntryView
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
	if len(rejected.After) != 0 {
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
	if len(second.Before) == 0 || !strings.Contains(string(second.Before), `"enabled":true`) {
		t.Fatalf("expected second entry's Before to reflect the prior enabled=true state, got %+v", second)
	}

	first := entries[1]
	if len(first.Before) != 0 {
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

// seedAuditEntryForTest performs a flag mutation so withAudit records an
// entry, then returns that entry as seen through the list endpoint.
func seedAuditEntryForTest(t *testing.T, mux *http.ServeMux, readToken string) auditEntryView {
	t.Helper()

	envID := seedEnvironmentForTest(t, "Production")
	writeToken := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite)

	key := fmt.Sprintf("audit-by-id-flag-%s", store.NewID())
	body, _ := json.Marshal(map[string]any{"key": key, "enabled": true, "environmentIds": []string{envID}})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+writeToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 setting flag, got %d: %s", rec.Code, rec.Body.String())
	}

	entries := getAudits(t, mux, readToken, "?targetType=flag&targetId="+key)
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 audit entry for %q, got %+v", key, entries)
	}
	return entries[0]
}

func TestAuditsGetByIDRequiresPermission(t *testing.T) {
	mux := newTestMux(t)
	seeded := seedAuditEntryForTest(t, mux, tokenFor(t, auth.PermAuditsRead))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/api/audits/%d", seeded.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without audits:read, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuditsGetByIDReturnsEntry(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)
	seeded := seedAuditEntryForTest(t, mux, token)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/api/audits/%d", seeded.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// after is a nested object, not a JSON string.
	if !strings.Contains(rec.Body.String(), `"after":{`) {
		t.Fatalf("expected after to be a nested JSON object, got %s", rec.Body.String())
	}

	var got auditEntryView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(got, seeded) {
		t.Fatalf("expected the by-id entry to match the listed entry\n got: %+v\nwant: %+v", got, seeded)
	}
}

func TestAuditsGetByIDReturnsNotFoundForUnknownID(t *testing.T) {
	mux := newTestMux(t)

	// Audit IDs are Raft log indexes, so MaxUint64 will never be reached.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/audits/18446744073709551615", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermAuditsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown audit id, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, CodeNotFoundAudit)
}

func TestAuditsGetByIDRejectsMalformedID(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)

	for _, id := range []string{"abc", "-1", "1.5", "18446744073709551616"} {
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/audits/"+id, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for malformed id %q, got %d: %s", id, rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, CodeBadRequestAuditIDInvalid)
		})
	}
}

// assertErrorBody checks rec carries the {"error", "code"} shape writeError
// produces with wantCode, so a 404/400 from the mux itself (plain text)
// doesn't pass.
func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, wantCode string) {
	t.Helper()

	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected a JSON error body, got %q: %v", rec.Body.String(), err)
	}
	if body.Error == "" || body.Code != wantCode {
		t.Fatalf("expected a non-empty error with code %q, got %+v", wantCode, body)
	}
}

// getAuditsPage is getAudits plus the pagination metadata.
func getAuditsPage(t *testing.T, mux *http.ServeMux, token string, query string) ([]auditEntryView, listPage) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/audits"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/audits%s, got %d: %s", query, rec.Code, rec.Body.String())
	}

	var payload struct {
		Audits []auditEntryView `json:"audits"`
		Page   listPage         `json:"page"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode audits response: %v", err)
	}
	return payload.Audits, payload.Page
}

// seedAuditEntriesForTest adds n entries under a fresh targetId and returns it.
func seedAuditEntriesForTest(t *testing.T, n int) string {
	t.Helper()

	targetID := "paged-" + store.NewID()
	for range n {
		if _, err := dataStore.Audits().Append(store.AuditEntry{ActorID: "pager", Action: "flag.set", TargetType: "flag", TargetID: targetID, Success: true, StatusCode: http.StatusOK}); err != nil {
			t.Fatalf("seed audit entry: %v", err)
		}
	}
	return targetID
}

func TestAuditsListPaginatesWithCursor(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)
	targetID := seedAuditEntriesForTest(t, defaultPageLimit+1)

	first, page := getAuditsPage(t, mux, token, "?targetId="+targetID)
	if len(first) != defaultPageLimit || page.Limit != defaultPageLimit || page.Total != defaultPageLimit+1 || page.NextCursor == "" {
		t.Fatalf("expected a full default page with a nextCursor, got %d entries, page %+v", len(first), page)
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].ID <= first[i].ID {
			t.Fatalf("expected newest-first order, got %d before %d", first[i-1].ID, first[i].ID)
		}
	}

	if page.Start != 1 || page.End != defaultPageLimit || page.PrevCursor != "" {
		t.Fatalf("expected the first page to cover rows 1-%d with no prevCursor, got %+v", defaultPageLimit, page)
	}

	rest, page := getAuditsPage(t, mux, token, "?targetId="+targetID+"&cursor="+page.NextCursor)
	if len(rest) != 1 || rest[0].ID >= first[len(first)-1].ID || page.NextCursor != "" || page.Total != defaultPageLimit+1 {
		t.Fatalf("expected the single remaining older entry and no nextCursor, got %+v, page %+v", rest, page)
	}
	if page.Start != defaultPageLimit+1 || page.End != defaultPageLimit+1 || page.PrevCursor == "" {
		t.Fatalf("expected the last page to be row %d with a prevCursor, got %+v", defaultPageLimit+1, page)
	}

	back, page := getAuditsPage(t, mux, token, "?targetId="+targetID+"&cursor="+page.PrevCursor)
	if len(back) != defaultPageLimit || back[0].ID != first[0].ID || page.Start != 1 || page.End != defaultPageLimit {
		t.Fatalf("expected prevCursor to lead back to the first page, got %d entries starting at %d, page %+v", len(back), back[0].ID, page)
	}

	limited, page := getAuditsPage(t, mux, token, "?targetId="+targetID+"&limit=10")
	if len(limited) != 10 || page.Limit != 10 {
		t.Fatalf("expected limit=10 to be honored, got %d entries, page %+v", len(limited), page)
	}

	// Cursor still respects filters.
	other, _ := getAuditsPage(t, mux, token, "?targetType=user&targetId="+targetID+"&cursor="+strconv.FormatUint(limited[0].ID, 10))
	if len(other) != 0 {
		t.Fatalf("expected no entries for a non-matching targetType, got %+v", other)
	}
}

func TestAuditsListFallsBackToDefaultLimit(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)
	targetID := seedAuditEntriesForTest(t, defaultPageLimit+1)

	for _, limit := range []string{"0", "-5", "101", "abc"} {
		t.Run(limit, func(t *testing.T) {
			entries, page := getAuditsPage(t, mux, token, "?targetId="+targetID+"&limit="+limit)
			if len(entries) != defaultPageLimit || page.Limit != defaultPageLimit {
				t.Fatalf("expected limit=%s to fall back to %d, got %d entries, page %+v", limit, defaultPageLimit, len(entries), page)
			}
		})
	}
}

func TestAuditsListRejectsMalformedCursor(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)

	for _, cursor := range []string{"abc", "-1", "1.5"} {
		t.Run(cursor, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/audits?cursor="+cursor, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for malformed cursor %q, got %d: %s", cursor, rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, CodeBadRequestAuditCursorInvalid)
		})
	}
}

func TestAuditsListFiltersByTimeWindow(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)
	targetID := seedAuditEntriesForTest(t, 3)

	format := func(t time.Time) string { return url.QueryEscape(t.Format(time.RFC3339)) }
	now := time.Now()
	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"window around now", "&from=" + format(now.Add(-time.Hour)) + "&to=" + format(now.Add(time.Hour)), 3},
		{"open-ended from", "&from=" + format(now.Add(-time.Hour)), 3},
		{"from in the future", "&from=" + format(now.Add(time.Hour)), 0},
		{"to in the past", "&to=" + format(now.Add(-time.Hour)), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entries, page := getAuditsPage(t, mux, token, "?targetId="+targetID+c.query)
			if len(entries) != c.want || page.Total != c.want {
				t.Fatalf("expected %d entries, got %d (total %d)", c.want, len(entries), page.Total)
			}
		})
	}
}

func TestAuditsListRejectsInvalidTimeWindow(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermAuditsRead)

	for _, query := range []string{"from=yesterday", "to=2026-10-09", "from=2026-10-09T12:00:00Z&to=2026-10-09T11:00:00Z"} {
		t.Run(query, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/audits?"+query, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %q, got %d: %s", query, rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, CodeBadRequestAuditTimeRangeInvalid)
		})
	}
}

func TestNestedJSON(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"{\"key\":\"a\"}\n": `{"key":"a"}`,
		`[1,2]`:             `[1,2]`,
		"not json":          `"not json"`,
	}
	for in, want := range cases {
		if got := string(nestedJSON(in)); got != want {
			t.Errorf("nestedJSON(%q) = %s, want %s", in, got, want)
		}
	}
}
