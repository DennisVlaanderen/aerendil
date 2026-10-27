package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// betaSegment is the fqdp.md §6.2 example.
func betaSegment(environmentID string) Segment {
	return Segment{
		EnvironmentID: environmentID,
		Key:           "beta",
		Name:          "Beta testers",
		Included:      []string{"user-1", "user-2"},
		Excluded:      []string{"user-9"},
		Rules: []SegmentRule{{
			ID:         "staff",
			Match:      "all",
			Conditions: []Condition{{Attribute: "email", Op: "endsWith", Values: []json.RawMessage{json.RawMessage(`"@example.com"`)}}},
		}},
	}
}

func TestStoreSegmentCRUD(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	applied, err := s.Segments().Set(betaSegment(envID))
	if err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	want := betaSegment(envID)
	want.SchemaVersion = 1
	want.EvaluatorLevel = 1
	want.Revision = s.raft.AppliedIndex()
	got, ok := s.Segments().Get(envID, "beta")
	if !ok || mustJSON(t, got) != mustJSON(t, want) || mustJSON(t, applied) != mustJSON(t, want) {
		t.Fatalf("stored segment differs:\n got %s\nwant %s", mustJSON(t, got), mustJSON(t, want))
	}
	if list := s.Segments().List(envID); len(list) != 1 {
		t.Fatalf("expected 1 segment in list, got %d", len(list))
	}

	if err := s.Segments().Delete(envID, "beta"); err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}
	if _, ok := s.Segments().Get(envID, "beta"); ok {
		t.Fatal("expected segment to be gone after delete")
	}
}

func TestStoreSetSegmentRejectsUnknownEnvironment(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Segments().Set(betaSegment("does-not-exist")); !errors.Is(err, ErrUnknownEnvironment) {
		t.Fatalf("expected ErrUnknownEnvironment, got %v", err)
	}
}

func TestStoreDeleteSegmentBlockedWhileReferenced(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")
	otherEnvID := seedEnvironment(t, s, "Staging")
	if _, err := s.Segments().Set(betaSegment(envID)); err != nil {
		t.Fatalf("set segment: %v", err)
	}
	if _, err := s.Segments().Set(betaSegment(otherEnvID)); err != nil {
		t.Fatalf("set segment: %v", err)
	}
	// richFlag references "beta" via inSegment.
	if _, err := s.Flags().Set(richFlag(envID)); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	if err := s.Segments().Delete(envID, "beta"); !errors.Is(err, ErrSegmentInUse) {
		t.Fatalf("expected ErrSegmentInUse, got %v", err)
	}
	if _, ok := s.Segments().Get(envID, "beta"); !ok {
		t.Fatal("expected referenced segment to survive")
	}
	// A reference in another environment doesn't block.
	if err := s.Segments().Delete(otherEnvID, "beta"); err != nil {
		t.Fatalf("expected delete in unreferenced environment to succeed: %v", err)
	}

	if err := s.Flags().Delete(envID, "checkout.new-flow"); err != nil {
		t.Fatalf("delete flag: %v", err)
	}
	if err := s.Segments().Delete(envID, "beta"); err != nil {
		t.Fatalf("expected delete to succeed once unreferenced: %v", err)
	}
}

func TestStoreDeleteEnvironmentRejectsWhenSegmentsStillReferenceIt(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")
	// A second environment isolates ErrEnvironmentHasSegments from ErrLastEnvironment.
	seedEnvironment(t, s, "Staging")
	if _, err := s.Segments().Set(betaSegment(envID)); err != nil {
		t.Fatalf("set segment: %v", err)
	}

	if err := s.Environments().Delete(envID); !errors.Is(err, ErrEnvironmentHasSegments) {
		t.Fatalf("expected ErrEnvironmentHasSegments, got %v", err)
	}
	// The FSM is the source of truth; the repository check is only a pre-check.
	s.fsm.mu.Lock()
	resp := s.fsm.applyEnvironment(0, command{Op: opDelete, Entity: entityEnvironment, Key: envID})
	s.fsm.mu.Unlock()
	if err, _ := resp.(error); !errors.Is(err, ErrEnvironmentHasSegments) {
		t.Fatalf("expected FSM to reject with ErrEnvironmentHasSegments, got %v", resp)
	}
	if _, ok := s.Environments().Get(envID); !ok {
		t.Fatal("expected the environment to still exist after rejected delete")
	}
}

func TestStoreSetSegmentEnforcesLimits(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	keys := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("user-%d", i)
		}
		return out
	}
	rules := func(n int) []SegmentRule {
		out := make([]SegmentRule, n)
		for i := range out {
			out[i] = SegmentRule{ID: fmt.Sprintf("r%d", i), Conditions: betaSegment(envID).Rules[0].Conditions}
		}
		return out
	}

	tests := []struct {
		name    string
		mutate  func(*Segment)
		wantErr error
	}{
		{"100 rules", func(seg *Segment) { seg.Rules = rules(100) }, nil},
		{"101 rules", func(seg *Segment) { seg.Rules = rules(101) }, ErrSegmentLimitExceeded},
		{"6000 keys", func(seg *Segment) { seg.Included, seg.Excluded = keys(3000), keys(3000) }, nil},
		{"6001 keys", func(seg *Segment) { seg.Included, seg.Excluded = keys(3001), keys(3000) }, ErrSegmentLimitExceeded},
		{"inSegment rule", func(seg *Segment) {
			seg.Rules[0].Conditions = []Condition{{Op: "inSegment", Values: []json.RawMessage{json.RawMessage(`"other"`)}}}
		}, ErrSegmentUsesInSegment},
		{"nested inSegment rule", func(seg *Segment) {
			seg.Rules[0].Conditions = []Condition{{Match: "any", Conditions: []Condition{{Op: "inSegment", Values: []json.RawMessage{json.RawMessage(`"other"`)}}}}}
		}, ErrSegmentUsesInSegment},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seg := betaSegment(envID)
			tt.mutate(&seg)
			_, err := s.Segments().Set(seg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFSMSnapshotRestoreRoundTripSegments(t *testing.T) {
	f := newFSM()
	f.environments["prod"] = Environment{ID: "prod", Name: "Production"}
	seg := betaSegment("prod")
	f.applySegment(7, command{Op: opSet, Entity: entitySegment, Segment: &seg})

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

	got, ok := restored.segments[flagMapKey("prod", "beta")]
	if !ok || mustJSON(t, got) != mustJSON(t, f.segments[flagMapKey("prod", "beta")]) || got.Revision != 7 {
		t.Fatalf("unexpected restored segment: %+v (ok=%v)", got, ok)
	}
}
