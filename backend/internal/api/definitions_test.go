package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/eval"
	"aerendil/backend/internal/store"
)

func doJSON(t *testing.T, mux *http.ServeMux, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func raws(values ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(values))
	for i, v := range values {
		out[i] = json.RawMessage(v)
	}
	return out
}

// richFlagForTest is the fqdp.md §6.1 example: a segment rule and a rollout.
func richFlagForTest(environmentID string) store.Flag {
	return store.Flag{
		EnvironmentID: environmentID,
		Key:           "checkout.new-flow",
		Type:          store.FlagTypeString,
		Enabled:       true,
		Variations: []store.Variation{
			{ID: "control", Value: json.RawMessage(`"v1"`)},
			{ID: "treatment", Value: json.RawMessage(`"v2"`)},
		},
		OffVariation: "control",
		Rules: []store.Rule{
			{
				ID:         "beta-testers",
				Match:      "all",
				Conditions: []store.Condition{{Op: "inSegment", Values: raws(`"beta"`)}},
				Serve:      store.Serve{Variation: "treatment"},
			},
			{
				ID:         "benelux-50",
				Match:      "any",
				Conditions: []store.Condition{{Attribute: "country", Op: "in", Values: raws(`"NL"`, `"BE"`, `"LU"`)}},
				Serve: store.Serve{Rollout: &store.Rollout{BucketBy: "targetingKey", Weights: []store.RolloutWeight{
					{Variation: "control", Weight: 50000}, {Variation: "treatment", Weight: 50000},
				}}},
			},
		},
		Fallthrough: store.Serve{Variation: "control"},
	}
}

// betaSegmentForTest is the fqdp.md §6.2 example.
func betaSegmentForTest(environmentID string) store.Segment {
	return store.Segment{
		EnvironmentID: environmentID,
		Key:           "beta",
		Name:          "Beta testers",
		Included:      []string{"user-1", "user-2"},
		Excluded:      []string{"user-9"},
		Rules: []store.SegmentRule{{
			ID:         "staff",
			Match:      "all",
			Conditions: []store.Condition{{Attribute: "email", Op: "endsWith", Values: raws(`"@example.com"`)}},
		}},
	}
}

func seedSegmentForTest(t *testing.T, segment store.Segment) {
	t.Helper()
	if _, err := dataStore.Segments().Set(segment); err != nil {
		t.Fatalf("seed segment: %v", err)
	}
}

func TestFlagsWriteValidationCodes(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedSegmentForTest(t, betaSegmentForTest(envID))
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite)

	rollout := func(f *store.Flag) *store.Rollout { return f.Rules[1].Serve.Rollout }
	leaf := func(f *store.Flag, c store.Condition) { f.Rules[1].Conditions = []store.Condition{c} }
	tests := []struct {
		name   string
		mutate func(*store.Flag)
		code   string
	}{
		{"key pattern", func(f *store.Flag) { f.Key = "-starts-with-dash" }, CodeBadRequestFlagKeyInvalid},
		{"unknown type", func(f *store.Flag) { f.Type = "float" }, CodeBadRequestFlagTypeInvalid},
		{"no variations", func(f *store.Flag) { f.Variations = nil }, CodeBadRequestFlagVariationsRequired},
		{"value does not fit type", func(f *store.Flag) { f.Variations[0].Value = json.RawMessage(`1`) }, CodeBadRequestFlagVariationValueInvalid},
		{"boolean variations not true, false", func(f *store.Flag) {
			f.Type = store.FlagTypeBoolean
			f.Variations[0].Value, f.Variations[1].Value = json.RawMessage(`true`), json.RawMessage(`false`)
		}, CodeBadRequestFlagBooleanVariations},
		{"offVariation unknown", func(f *store.Flag) { f.OffVariation = "nope" }, CodeBadRequestFlagVariationUnknown},
		{"serve variation unknown", func(f *store.Flag) { f.Fallthrough.Variation = "nope" }, CodeBadRequestFlagVariationUnknown},
		{"rollout variation unknown", func(f *store.Flag) { rollout(f).Weights[0].Variation = "nope" }, CodeBadRequestFlagVariationUnknown},
		{"rollout variation listed twice", func(f *store.Flag) { rollout(f).Weights[1].Variation = "control" }, CodeBadRequestFlagVariationUnknown},
		{"serve with both", func(f *store.Flag) { f.Fallthrough.Rollout = rollout(f) }, CodeBadRequestFlagServeInvalid},
		{"weights not summing to 100000", func(f *store.Flag) { rollout(f).Weights[0].Weight = 1 }, CodeBadRequestFlagRolloutWeightsInvalid},
		{"weight out of range", func(f *store.Flag) {
			rollout(f).Weights[0].Weight, rollout(f).Weights[1].Weight = -1, 100001
		}, CodeBadRequestFlagRolloutWeightsInvalid},
		{"variation id pattern", func(f *store.Flag) { f.Variations[1].ID = "has space" }, CodeBadRequestDefinitionIDInvalid},
		{"rule id pattern", func(f *store.Flag) { f.Rules[1].ID = "" }, CodeBadRequestDefinitionIDInvalid},
		{"duplicate variation id", func(f *store.Flag) { f.Variations[1].ID = "control" }, CodeBadRequestDefinitionIDDuplicate},
		{"duplicate rule id", func(f *store.Flag) { f.Rules[1].ID = "beta-testers" }, CodeBadRequestDefinitionIDDuplicate},
		{"too many rules", func(f *store.Flag) {
			for i := range maxFlagRules {
				f.Rules = append(f.Rules, store.Rule{ID: "r" + strings.Repeat("x", i%60), Conditions: f.Rules[1].Conditions, Serve: f.Fallthrough})
			}
		}, CodeBadRequestDefinitionLimitExceeded},
		{"too many values", func(f *store.Flag) {
			values := make([]json.RawMessage, maxConditionValues+1)
			for i := range values {
				values[i] = json.RawMessage(`"x"`)
			}
			leaf(f, store.Condition{Attribute: "a", Op: "in", Values: values})
		}, CodeBadRequestDefinitionLimitExceeded},
		{"unknown match", func(f *store.Flag) { f.Rules[1].Match = "none" }, CodeBadRequestConditionGroupInvalid},
		{"empty group", func(f *store.Flag) { f.Rules[1].Conditions = nil }, CodeBadRequestConditionGroupInvalid},
		{"nested group", func(f *store.Flag) {
			leaf(f, store.Condition{Match: "all", Conditions: f.Rules[1].Conditions})
		}, CodeBadRequestConditionGroupNested},
		{"unknown op", func(f *store.Flag) { leaf(f, store.Condition{Attribute: "a", Op: "matches", Values: raws(`"x"`)}) }, CodeBadRequestConditionInvalid},
		{"mixed value kinds", func(f *store.Flag) { leaf(f, store.Condition{Attribute: "a", Op: "in", Values: raws(`"x"`, `1`)}) }, CodeBadRequestConditionValuesInvalid},
		{"Inf value", func(f *store.Flag) { leaf(f, store.Condition{Attribute: "a", Op: "gt", Values: raws(`1e400`)}) }, CodeBadRequestConditionValuesInvalid},
		{"unknown segment", func(f *store.Flag) { f.Rules[0].Conditions[0].Values = raws(`"gamma"`) }, CodeBadRequestSegmentUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := richFlagForTest("")
			tt.mutate(&flag)
			body := jsonBody(struct {
				store.Flag
				EnvironmentIDs []string `json:"environmentIds"`
			}{flag, []string{envID}})
			rec := doJSON(t, mux, http.MethodPost, "/api/flags", token, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, tt.code)
		})
	}
	if list := dataStore.Flags().List(envID); len(list) != 0 {
		t.Fatalf("expected no flag to be written, got %d", len(list))
	}
}

func TestFlagsPutRejectsTypeChange(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsUpdate)
	if _, err := dataStore.Flags().Set(testFlag(envID, "checkout", true, "on")); err != nil {
		t.Fatalf("seed flag: %v", err)
	}

	flag := testFlag("", "", true, "")
	flag.Type = store.FlagTypeJSON
	rec := doJSON(t, mux, http.MethodPut, "/api/flags/checkout?environmentId="+envID, token, jsonBody(flag))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, CodeBadRequestFlagTypeChanged)
}

func TestFlagsRevisionReturnedOnReadsAndIgnoredOnWrites(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedSegmentForTest(t, betaSegmentForTest(envID))
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite, auth.PermFlagsUpdate)

	flag := richFlagForTest("")
	flag.Revision, flag.SchemaVersion, flag.EvaluatorLevel = 999999, 7, 9
	body := jsonBody(struct {
		store.Flag
		EnvironmentIDs []string `json:"environmentIds"`
	}{flag, []string{envID}})
	if rec := doJSON(t, mux, http.MethodPost, "/api/flags", token, body); rec.Code != http.StatusOK {
		t.Fatalf("expected create to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, mux, http.MethodGet, "/api/flags/checkout.new-flow?environmentId="+envID, token, nil)
	var got store.Flag
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Revision == 0 || got.Revision == 999999 || got.SchemaVersion != 1 || got.EvaluatorLevel != 1 {
		t.Fatalf("expected store-owned fields from the store, got revision %d, schemaVersion %d, evaluatorLevel %d",
			got.Revision, got.SchemaVersion, got.EvaluatorLevel)
	}
	if len(got.Rules) != 2 || got.Rules[1].Serve.Rollout == nil || got.Salt != "checkout.new-flow" {
		t.Fatalf("expected the full §6.1 definition back, got %s", rec.Body.String())
	}

	// A PUT that omits or changes the salt must not reshuffle rollout buckets.
	for _, salt := range []string{"", "other-salt"} {
		update := richFlagForTest("")
		update.Salt = salt
		path := "/api/flags/checkout.new-flow?environmentId=" + envID
		rec = doJSON(t, mux, http.MethodPut, path, token, jsonBody(update))
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT with salt %q: expected 200, got %d: %s", salt, rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Salt != "checkout.new-flow" {
			t.Fatalf("PUT with salt %q changed the stored salt to %q", salt, got.Salt)
		}
	}
}

func TestFlagsAuditRecordsRuleChanges(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedSegmentForTest(t, betaSegmentForTest(envID))
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsWrite, auth.PermFlagsUpdate, auth.PermFlagsDelete)

	create := jsonBody(struct {
		store.Flag
		EnvironmentIDs []string `json:"environmentIds"`
	}{richFlagForTest(""), []string{envID}})
	if rec := doJSON(t, mux, http.MethodPost, "/api/flags", token, create); rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	// Replace beta-testers with a new staff rule, and change benelux-50.
	update := richFlagForTest("")
	update.Rules[1].Conditions[0].Values = raws(`"NL"`)
	update.Rules[0] = store.Rule{
		ID:         "staff",
		Conditions: []store.Condition{{Attribute: "email", Op: "endsWith", Values: raws(`"@example.com"`)}},
		Serve:      store.Serve{Variation: "treatment"},
	}
	path := "/api/flags/checkout.new-flow?environmentId=" + envID
	if rec := doJSON(t, mux, http.MethodPut, path, token, jsonBody(update)); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, mux, http.MethodDelete, path, token, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}

	want := map[string]string{
		"flag.set":    `{"rules":{"added":["beta-testers","benelux-50"]}}`,
		"flag.update": `{"rules":{"added":["staff"],"changed":["benelux-50"],"removed":["beta-testers"]}}`,
		"flag.delete": `{"rules":{"removed":["staff","benelux-50"]}}`,
	}
	for _, entry := range dataStore.Audits().List(store.AuditFilter{TargetType: "flag"}) {
		if w, ok := want[entry.Action]; ok {
			if entry.Details != w {
				t.Errorf("%s details: got %s, want %s", entry.Action, entry.Details, w)
			}
			delete(want, entry.Action)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing audit entries: %v", want)
	}
}

func TestFlagsEvaluate(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Production")
	seedSegmentForTest(t, betaSegmentForTest(envID))
	if _, err := dataStore.Flags().Set(richFlagForTest(envID)); err != nil {
		t.Fatalf("seed flag: %v", err)
	}
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead)
	path := "/api/flags/checkout.new-flow/evaluate?environmentId=" + envID

	tests := []struct {
		name string
		ctx  string
		want eval.Result
	}{
		{"segment member", `{"targetingKey":"user-1"}`,
			eval.Result{Value: json.RawMessage(`"v2"`), VariationID: "treatment", Reason: eval.ReasonRuleMatch, RuleID: "beta-testers"}},
		{"§6.5 vector user-123", `{"targetingKey":"user-123","attributes":{"country":"NL"}}`,
			eval.Result{Value: json.RawMessage(`"v2"`), VariationID: "treatment", Reason: eval.ReasonRuleMatch, RuleID: "benelux-50"}},
		{"§6.5 vector user-456", `{"targetingKey":"user-456","attributes":{"country":"NL"}}`,
			eval.Result{Value: json.RawMessage(`"v1"`), VariationID: "control", Reason: eval.ReasonRuleMatch, RuleID: "benelux-50"}},
		{"fallthrough", `{"targetingKey":"user-5","attributes":{"country":"DE"}}`,
			eval.Result{Value: json.RawMessage(`"v1"`), VariationID: "control", Reason: eval.ReasonFallthrough}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(t, mux, http.MethodPost, path, token, []byte(tt.ctx))
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if got, want := strings.TrimSpace(rec.Body.String()), string(jsonBody(tt.want)); got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}

	for name, ctx := range map[string]string{
		"no targetingKey":         `{}`,
		"reserved attribute":      `{"targetingKey":"k","attributes":{"targetingKey":"x"}}`,
		"bad attribute name":      `{"targetingKey":"k","attributes":{"a b":"x"}}`,
		"mixed list":              `{"targetingKey":"k","attributes":{"a":["x",1]}}`,
		"nested object":           `{"targetingKey":"k","attributes":{"a":{"b":1}}}`,
		"null attribute":          `{"targetingKey":"k","attributes":{"a":null}}`,
		"targetingKey over limit": `{"targetingKey":"` + strings.Repeat("k", maxTargetingKeyBytes+1) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, mux, http.MethodPost, path, token, []byte(ctx))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, CodeBadRequestFlagContextInvalid)
		})
	}

	rec := doJSON(t, mux, http.MethodPost, "/api/flags/missing/evaluate?environmentId="+envID, token, []byte(`{"targetingKey":"k"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown flag, got %d", rec.Code)
	}
	noAccess := tokenFor(t, auth.PermFlagsRead)
	if rec := doJSON(t, mux, http.MethodPost, path, noAccess, []byte(`{"targetingKey":"k"}`)); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without environment access, got %d", rec.Code)
	}
}

// TestStoreFlagConvertsToEvalFlag guards the JSON round trip toEval relies
// on: every member eval reads must come through with the store's value.
func TestStoreFlagConvertsToEvalFlag(t *testing.T) {
	stored := richFlagForTest("prod")
	stored.Salt = "custom-salt"
	stored.Rules[1].Conditions[0].Negate = true
	converted, err := toEval[eval.Flag](stored)
	if err != nil {
		t.Fatal(err)
	}

	var storeJSON, evalJSON map[string]any
	_ = json.Unmarshal(jsonBody(stored), &storeJSON)
	_ = json.Unmarshal(jsonBody(converted), &evalJSON)
	for member, value := range evalJSON {
		if string(jsonBody(storeJSON[member])) != string(jsonBody(value)) {
			t.Errorf("member %q: store %s, eval %s", member, jsonBody(storeJSON[member]), jsonBody(value))
		}
	}
	if len(evalJSON) != 9 {
		t.Errorf("expected eval.Flag's 9 members to survive, got %d: %v", len(evalJSON), evalJSON)
	}
}
