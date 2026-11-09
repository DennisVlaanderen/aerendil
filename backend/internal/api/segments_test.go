package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func TestSegmentsCRUD(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID},
		auth.PermSegmentsRead, auth.PermSegmentsWrite, auth.PermSegmentsUpdate, auth.PermSegmentsDelete)
	path := "/api/segments/beta?environmentId=" + envID

	segment := betaSegmentForTest(envID)
	segment.Revision = 999999 // ignored on write
	rec := doJSON(t, mux, http.MethodPost, "/api/segments", token, jsonBody(segment))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected create to return 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, mux, http.MethodPost, "/api/segments", token, jsonBody(segment)); rec.Code != http.StatusConflict {
		t.Fatalf("expected a second create to return 409, got %d: %s", rec.Code, rec.Body.String())
	} else {
		assertErrorBody(t, rec, CodeConflictSegmentExists)
	}

	rec = doJSON(t, mux, http.MethodGet, path, token, nil)
	var got store.Segment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if got.Revision == 0 || got.Revision == 999999 || got.Name != "Beta testers" || len(got.Rules) != 1 {
		t.Fatalf("unexpected segment: %s", rec.Body.String())
	}

	update := betaSegmentForTest("")
	update.Included = []string{"user-3"}
	rec = doJSON(t, mux, http.MethodPut, path, token, jsonBody(update))
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, mux, http.MethodGet, "/api/segments?environmentId="+envID, token, nil)
	var listed struct {
		Segments []store.Segment `json:"segments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed.Segments) != 1 ||
		len(listed.Segments[0].Included) != 1 || listed.Segments[0].Included[0] != "user-3" {
		t.Fatalf("expected the update in the list, got %s", rec.Body.String())
	}

	if rec := doJSON(t, mux, http.MethodDelete, path, token, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		rec := doJSON(t, mux, method, path, token, jsonBody(update))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s after delete: expected 404, got %d", method, rec.Code)
		}
		assertErrorBody(t, rec, CodeNotFoundSegment)
	}
}

func TestSegmentsDeleteRejectedWhileReferenced(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedSegmentForTest(t, betaSegmentForTest(envID))
	if _, err := dataStore.Flags().Set(richFlagForTest(envID)); err != nil {
		t.Fatalf("seed flag: %v", err)
	}
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermSegmentsDelete)

	rec := doJSON(t, mux, http.MethodDelete, "/api/segments/beta?environmentId="+envID, token, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, CodeConflictSegmentInUse)
	if _, ok := dataStore.Segments().Get(envID, "beta"); !ok {
		t.Fatal("expected the referenced segment to survive")
	}
}

func TestSegmentsPermissions(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedSegmentForTest(t, betaSegmentForTest(envID))
	path := "/api/segments/beta?environmentId=" + envID
	body := jsonBody(betaSegmentForTest(envID))

	tests := []struct {
		method, path, perm string
	}{
		{http.MethodGet, "/api/segments?environmentId=" + envID, auth.PermSegmentsRead},
		{http.MethodGet, path, auth.PermSegmentsRead},
		{http.MethodPost, "/api/segments", auth.PermSegmentsWrite},
		{http.MethodPut, path, auth.PermSegmentsUpdate},
		{http.MethodDelete, path, auth.PermSegmentsDelete},
	}
	for _, tt := range tests {
		t.Run(tt.method+" needs "+tt.perm, func(t *testing.T) {
			// Every other segments permission, but not this one.
			var others []string
			for _, p := range []string{auth.PermSegmentsRead, auth.PermSegmentsWrite, auth.PermSegmentsUpdate, auth.PermSegmentsDelete} {
				if p != tt.perm {
					others = append(others, p)
				}
			}
			if rec := doJSON(t, mux, tt.method, tt.path, tokenForWithEnvironments(t, []string{envID}, others...), body); rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 without %s, got %d", tt.perm, rec.Code)
			}
			if rec := doJSON(t, mux, tt.method, tt.path, tokenFor(t, tt.perm), body); rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 without environment access, got %d", rec.Code)
			}
		})
	}
}

func TestSegmentsWriteValidationCodes(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermSegmentsWrite)

	tests := []struct {
		name   string
		mutate func(*store.Segment)
		code   string
	}{
		{"environmentId required", func(s *store.Segment) { s.EnvironmentID = "" }, CodeBadRequestSegmentEnvironmentIDRequired},
		{"key pattern", func(s *store.Segment) { s.Key = "has space" }, CodeBadRequestSegmentKeyInvalid},
		{"rule id pattern", func(s *store.Segment) { s.Rules[0].ID = "" }, CodeBadRequestDefinitionIDInvalid},
		{"duplicate rule id", func(s *store.Segment) { s.Rules = append(s.Rules, s.Rules[0]) }, CodeBadRequestDefinitionIDDuplicate},
		{"uses inSegment", func(s *store.Segment) {
			s.Rules[0].Conditions = []store.Condition{{Op: "inSegment", Values: raws(`"other"`)}}
		}, CodeBadRequestSegmentUsesInSegment},
		{"invalid condition", func(s *store.Segment) { s.Rules[0].Conditions[0].Values = nil }, CodeBadRequestConditionInvalid},
		{"too many targeting keys", func(s *store.Segment) { s.Included = make([]string, 6001) }, CodeBadRequestDefinitionLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segment := betaSegmentForTest(envID)
			tt.mutate(&segment)
			rec := doJSON(t, mux, http.MethodPost, "/api/segments", token, jsonBody(segment))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, tt.code)
		})
	}
}
