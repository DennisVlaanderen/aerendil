package store

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestFSMSnapshotRestoreRoundTripAllEntities(t *testing.T) {
	f := newFSM()
	f.flags["a"] = Flag{Key: "a", Enabled: true, Version: 1}
	f.users["u1"] = User{ID: "u1", Username: "alice", GroupIDs: []string{"editors"}, Active: true, Version: 2}
	f.groups["editors"] = Group{ID: "editors", Name: "Editors", Permissions: []string{"flags:read"}, Version: 3}
	f.groups[AdminGroupID] = Group{ID: AdminGroupID, Name: "Admin", System: true, Version: 4}
	f.auditEntries[5] = AuditEntry{ID: 5, Timestamp: 1234, ActorID: "u1", Action: "flag.set", TargetType: "flag", TargetID: "a", Success: true, StatusCode: 200}

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

	if len(restored.flags) != 1 || !restored.flags["a"].Enabled {
		t.Fatalf("unexpected restored flags: %+v", restored.flags)
	}

	restoredUser, ok := restored.users["u1"]
	if !ok || restoredUser.Username != "alice" || len(restoredUser.GroupIDs) != 1 {
		t.Fatalf("unexpected restored users: %+v (ok=%v)", restoredUser, ok)
	}

	if len(restored.groups) != 2 {
		t.Fatalf("expected 2 restored groups, got %d", len(restored.groups))
	}
	restoredAdmin, ok := restored.groups[AdminGroupID]
	if !ok || !restoredAdmin.System {
		t.Fatalf("expected Admin group to round-trip with System=true, got %+v (ok=%v)", restoredAdmin, ok)
	}

	restoredEntry, ok := restored.auditEntries[5]
	if !ok || restoredEntry.Action != "flag.set" || restoredEntry.TargetID != "a" {
		t.Fatalf("unexpected restored audit entries: %+v (ok=%v)", restoredEntry, ok)
	}
}

// TestFSMRestoreRejectsPreUserGroupSnapshotFormat guards against silently
// discarding pre-existing flags on upgrade: an old snapshot was a flat
// map[string]Flag with no "flags"/"users"/"groups" top-level keys, which
// (before this test's fix) decoded "successfully" into three nil maps with
// no error, silently losing every previously stored flag.
func TestFSMRestoreRejectsPreUserGroupSnapshotFormat(t *testing.T) {
	f := newFSM()
	legacy := `{"checkout": {"key": "checkout", "enabled": true, "version": 1}}`
	err := f.Restore(io.NopCloser(strings.NewReader(legacy)))
	if err == nil {
		t.Fatal("expected Restore to reject an unrecognized pre-user/group snapshot format")
	}
}

// TestFSMRestoreToleratesEmptySnapshot guards the legitimate case Restore
// must still accept: a brand-new node with no snapshot taken yet.
func TestFSMRestoreToleratesEmptySnapshot(t *testing.T) {
	f := newFSM()
	if err := f.Restore(io.NopCloser(strings.NewReader(""))); err != nil {
		t.Fatalf("expected Restore to tolerate an empty snapshot, got %v", err)
	}
	if f.flags == nil || f.users == nil || f.groups == nil || f.auditEntries == nil {
		t.Fatalf("expected all maps to default to empty, got flags=%v users=%v groups=%v auditEntries=%v", f.flags, f.users, f.groups, f.auditEntries)
	}
}

// TestApplyAuditEntrySetsIDFromLogIndex guards the Raft-determinism
// requirement that ID (like Flag.Version) comes from the log index every
// node applies identically, while Timestamp is passed through unchanged --
// it must be assigned once by the leader-side AuditRepository.Append, never
// inside applyAuditEntry (which every replica re-executes independently).
func TestApplyAuditEntrySetsIDFromLogIndex(t *testing.T) {
	f := newFSM()

	resp := f.applyAuditEntry(7, command{Op: opAppend, Entity: entityAudit, AuditEntry: &AuditEntry{
		Timestamp:  1700000000,
		ActorID:    "u1",
		Action:     "user.create",
		TargetType: "user",
		TargetID:   "u2",
		Success:    true,
		StatusCode: 201,
	}})

	applied, ok := resp.(AuditEntry)
	if !ok {
		t.Fatalf("expected applyAuditEntry to return an AuditEntry, got %+v", resp)
	}
	if applied.ID != 7 {
		t.Fatalf("expected ID to be set from the log index (7), got %d", applied.ID)
	}
	if applied.Timestamp != 1700000000 {
		t.Fatalf("expected Timestamp to pass through unchanged, got %d", applied.Timestamp)
	}

	stored, ok := f.auditEntries[7]
	if !ok || stored.ActorID != "u1" || stored.TargetID != "u2" {
		t.Fatalf("unexpected stored audit entry: %+v (ok=%v)", stored, ok)
	}
}

func TestFSMApplyRejectsAdminGroupMutationEvenIfSuccessfullyRaftCommitted(t *testing.T) {
	f := newFSM()
	f.groups[AdminGroupID] = Group{ID: AdminGroupID, Name: "Admin", System: true, Version: 1}

	resp := f.applyGroup(2, command{Op: opSet, Entity: entityGroup, Group: &Group{ID: AdminGroupID, Name: "Renamed"}})
	if _, ok := resp.(error); !ok {
		t.Fatalf("expected applyGroup to return an error for a System group edit, got %+v", resp)
	}

	resp = f.applyGroup(3, command{Op: opDelete, Entity: entityGroup, Key: AdminGroupID})
	if _, ok := resp.(error); !ok {
		t.Fatalf("expected applyGroup to return an error for a System group delete, got %+v", resp)
	}

	if got := f.groups[AdminGroupID]; got.Name != "Admin" {
		t.Fatalf("expected Admin group to be unchanged, got %+v", got)
	}
}

func TestApplyUserRejectsDuplicateUsername(t *testing.T) {
	f := newFSM()
	f.users["u1"] = User{ID: "u1", Username: "alice", Active: true, Version: 1}

	resp := f.applyUser(2, command{Op: opSet, Entity: entityUser, User: &User{ID: "u2", Username: "alice"}})
	respErr, ok := resp.(error)
	if !ok {
		t.Fatalf("expected applyUser to return an error for a duplicate username, got %+v", resp)
	}
	if !errors.Is(respErr, ErrUsernameTaken) {
		t.Fatalf("expected error to be ErrUsernameTaken, got %v", respErr)
	}
	if _, exists := f.users["u2"]; exists {
		t.Fatal("expected the conflicting user to not be written")
	}
}

func TestApplyUserAllowsUpdatingSameUserWithSameUsername(t *testing.T) {
	f := newFSM()
	f.users["u1"] = User{ID: "u1", Username: "alice", Active: true, Version: 1}

	resp := f.applyUser(2, command{Op: opSet, Entity: entityUser, User: &User{ID: "u1", Username: "alice", Active: false}})
	updated, ok := resp.(User)
	if !ok {
		t.Fatalf("expected applyUser to succeed updating the same user, got %+v", resp)
	}
	if updated.Active {
		t.Fatalf("expected the update to be applied, got %+v", updated)
	}
}
