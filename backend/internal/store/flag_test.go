package store

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// stringFlag is a minimal valid definition: one "on" variation served
// everywhere.
func stringFlag(environmentID, key string, enabled bool) Flag {
	return Flag{
		EnvironmentID: environmentID,
		Key:           key,
		Type:          FlagTypeString,
		Enabled:       enabled,
		Variations:    []Variation{{ID: "on", Value: json.RawMessage(`"on"`)}},
		OffVariation:  "on",
		Fallthrough:   Serve{Variation: "on"},
	}
}

// richFlag is the fqdp.md §6.1 example: a segment rule and a rollout rule.
func richFlag(environmentID string) Flag {
	return Flag{
		EnvironmentID: environmentID,
		Key:           "checkout.new-flow",
		Type:          FlagTypeString,
		Enabled:       true,
		Salt:          "custom-salt",
		Variations: []Variation{
			{ID: "control", Value: json.RawMessage(`"v1"`)},
			{ID: "treatment", Value: json.RawMessage(`"v2"`)},
		},
		OffVariation: "control",
		Rules: []Rule{
			{
				ID:          "beta-testers",
				Description: "Everyone in the beta segment",
				Match:       "all",
				Conditions:  []Condition{{Op: "inSegment", Values: []json.RawMessage{json.RawMessage(`"beta"`)}}},
				Serve:       Serve{Variation: "treatment"},
			},
			{
				ID:         "benelux-50",
				Match:      "any",
				Conditions: []Condition{{Attribute: "country", Op: "in", Values: []json.RawMessage{json.RawMessage(`"NL"`), json.RawMessage(`"BE"`), json.RawMessage(`"LU"`)}, Negate: true}},
				Serve: Serve{Rollout: &Rollout{
					BucketBy: "targetingKey",
					Weights: []RolloutWeight{
						{Variation: "control", Weight: 50000},
						{Variation: "treatment", Weight: 50000},
					},
				}},
			},
		},
		Fallthrough: Serve{Variation: "control"},
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestStoreSetFlagKeepsRulesAndRollouts(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	applied, err := s.Flags().Set(richFlag(envID))
	if err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	if applied.Revision != s.raft.AppliedIndex() {
		t.Fatalf("expected revision %d to equal the raft index %d", applied.Revision, s.raft.AppliedIndex())
	}

	want := richFlag(envID)
	want.SchemaVersion = 1
	want.EvaluatorLevel = 1
	want.Revision = applied.Revision
	got, ok := s.Flags().Get(envID, "checkout.new-flow")
	if !ok {
		t.Fatal("expected flag to be present after set")
	}
	if mustJSON(t, got) != mustJSON(t, want) {
		t.Fatalf("stored flag differs:\n got %s\nwant %s", mustJSON(t, got), mustJSON(t, want))
	}
}

func TestStoreSetFlagKeepsLargeIntegerConditionValues(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	flag := stringFlag(envID, "checkout", true)
	flag.Rules = []Rule{{
		ID:         "one-user",
		Conditions: []Condition{{Attribute: "userId", Op: "in", Values: []json.RawMessage{json.RawMessage(`9007199254740993`)}}},
		Serve:      Serve{Variation: "on"},
	}}
	if _, err := s.Flags().Set(flag); err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	got, _ := s.Flags().Get(envID, "checkout")
	if v := string(got.Rules[0].Conditions[0].Values[0]); v != "9007199254740993" {
		t.Fatalf("expected the condition value to survive unrounded, got %s", v)
	}
}

func TestStoreSetFlagDefaultsSaltToKey(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	applied, err := s.Flags().Set(stringFlag(envID, "checkout", true))
	if err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	if applied.Salt != "checkout" {
		t.Fatalf("expected salt to default to the key, got %q", applied.Salt)
	}
}

func TestStoreSetFlagRejectsTypeChange(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	if _, err := s.Flags().Set(stringFlag(envID, "checkout", true)); err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	retyped := stringFlag(envID, "checkout", false)
	retyped.Type = FlagTypeBoolean
	if _, err := s.Flags().Set(retyped); !errors.Is(err, ErrFlagTypeChanged) {
		t.Fatalf("expected ErrFlagTypeChanged, got %v", err)
	}
	if got, _ := s.Flags().Get(envID, "checkout"); got.Type != FlagTypeString || !got.Enabled {
		t.Fatalf("expected the flag to be unchanged, got %+v", got)
	}
}

func TestStoreSetFlagRejectsUnknownType(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	flag := stringFlag(envID, "checkout", true)
	flag.Type = "float"
	if _, err := s.Flags().Set(flag); !errors.Is(err, ErrUnknownFlagType) {
		t.Fatalf("expected ErrUnknownFlagType from Set, got %v", err)
	}
	if _, err := s.Flags().SetMany(flag, []string{envID}); !errors.Is(err, ErrUnknownFlagType) {
		t.Fatalf("expected ErrUnknownFlagType from SetMany, got %v", err)
	}
}

func TestStoreSetManyKeepsRulesAndRollouts(t *testing.T) {
	s := newTestStore(t)
	envA := seedEnvironment(t, s, "A")
	envB := seedEnvironment(t, s, "B")

	if _, err := s.Flags().SetMany(richFlag("ignored"), []string{envA, envB}); err != nil {
		t.Fatalf("expected SetMany to succeed: %v", err)
	}
	for _, envID := range []string{envA, envB} {
		got, ok := s.Flags().Get(envID, "checkout.new-flow")
		if !ok || got.EnvironmentID != envID || len(got.Rules) != 2 || got.Rules[1].Serve.Rollout == nil {
			t.Fatalf("expected the full definition in %q, got %+v (ok=%v)", envID, got, ok)
		}
	}
}

func TestStoreDeleteFlag(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	if _, err := s.Flags().Set(richFlag(envID)); err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	if err := s.Flags().Delete(envID, "checkout.new-flow"); err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}
	if _, ok := s.Flags().Get(envID, "checkout.new-flow"); ok {
		t.Fatal("expected flag to be gone after delete")
	}
}

func TestFSMSnapshotRestoreRoundTripFlagDefinition(t *testing.T) {
	f := newFSM()
	f.environments["prod"] = Environment{ID: "prod", Name: "Production"}
	flag := richFlag("prod")
	if err, isErr := f.applyFlag(7, command{Op: opSet, Entity: entityFlag, Flag: &flag}).(error); isErr {
		t.Fatalf("expected applyFlag to succeed: %v", err)
	}
	want := f.flags[flagMapKey("prod", "checkout.new-flow")]

	snap, err := f.Snapshot()
	if err != nil {
		t.Fatalf("expected snapshot to succeed: %v", err)
	}
	sink := &memSnapshotSink{}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("expected persist to succeed: %v", err)
	}
	restored := newFSM()
	if err := restored.Restore(sink.reader()); err != nil {
		t.Fatalf("expected restore to succeed: %v", err)
	}

	got := restored.flags[flagMapKey("prod", "checkout.new-flow")]
	if mustJSON(t, got) != mustJSON(t, want) {
		t.Fatalf("restored flag differs:\n got %s\nwant %s", mustJSON(t, got), mustJSON(t, want))
	}
	if got.Revision != 7 {
		t.Fatalf("expected revision 7 to survive the round trip, got %d", got.Revision)
	}
}

// An old {enabled, value} flag decodes with an empty Type; refuse to boot
// on it rather than serve a definition with no variations.
func TestFSMRestoreRejectsPreDefinitionFlagSnapshot(t *testing.T) {
	f := newFSM()
	oldShape := `{"flags": {"prod/a": {"environmentId": "prod", "key": "a", "enabled": true, "value": "on", "version": 1}}, "users": {}, "groups": {}, "auditEntries": {}, "environments": {}}`
	if err := f.Restore(io.NopCloser(strings.NewReader(oldShape))); err == nil {
		t.Fatal("expected Restore to reject a snapshot with pre-definition flags")
	}
}
