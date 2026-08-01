package store

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestFSMSnapshotRestoreRoundTripAllEntities(t *testing.T) {
	f := newFSM()
	f.flags[flagMapKey("prod", "a")] = Flag{EnvironmentID: "prod", Key: "a", Enabled: true, Version: 1}
	f.users["u1"] = User{ID: "u1", Username: "alice", GroupIDs: []string{"editors"}, Active: true, Version: 2}
	f.groups["editors"] = Group{ID: "editors", Name: "Editors", Permissions: []string{"flags:read"}, Version: 3}
	f.groups[AdminGroupID] = Group{ID: AdminGroupID, Name: "Admin", System: true, Version: 4}
	f.auditEntries[5] = AuditEntry{ID: 5, Timestamp: 1234, ActorID: "u1", Action: "flag.set", TargetType: "flag", TargetID: "a", Success: true, StatusCode: 200}
	f.environments["prod"] = Environment{ID: "prod", Name: "Production", Order: 0, Version: 6}

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

	restoredFlag, ok := restored.flags[flagMapKey("prod", "a")]
	if !ok || !restoredFlag.Enabled || restoredFlag.EnvironmentID != "prod" {
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

	restoredEnv, ok := restored.environments["prod"]
	if !ok || restoredEnv.Name != "Production" || restoredEnv.Version != 6 {
		t.Fatalf("unexpected restored environments: %+v (ok=%v)", restoredEnv, ok)
	}
}

// TestFSMRestoreToleratesSnapshotWithoutEnvironmentsKey guards the upgrade
// path: an existing snapshot written before Environment existed has
// "flags"/"users"/"groups" but no "environments" key at all. That must
// restore cleanly with environments defaulting to empty, not be rejected by
// the pre-user/group legacy-format check (which is about detecting the much
// older flat-map-only format, not about requiring every field that will
// ever exist).
func TestFSMRestoreToleratesSnapshotWithoutEnvironmentsKey(t *testing.T) {
	f := newFSM()
	preEnvironmentSnapshot := `{"flags": {"legacy-env/a": {"environmentId": "legacy-env", "key": "a", "enabled": true, "version": 1}}, "users": {}, "groups": {}, "auditEntries": {}}`
	if err := f.Restore(io.NopCloser(strings.NewReader(preEnvironmentSnapshot))); err != nil {
		t.Fatalf("expected Restore to tolerate a snapshot without an environments key, got %v", err)
	}
	if f.environments == nil || len(f.environments) != 0 {
		t.Fatalf("expected environments to default to an empty map, got %+v", f.environments)
	}
	if !f.flags[flagMapKey("legacy-env", "a")].Enabled {
		t.Fatalf("expected pre-existing flags to still restore correctly, got %+v", f.flags)
	}
}

// TestFSMRestoreRejectsPreEnvironmentScopedFlagSnapshot guards against
// silently orphaning existing flags under environment ID "" forever: a
// snapshot written before Flag.EnvironmentID existed decodes "successfully"
// (the new field just zero-values to "") but must be rejected instead of
// silently losing every flag's environment association.
func TestFSMRestoreRejectsPreEnvironmentScopedFlagSnapshot(t *testing.T) {
	f := newFSM()
	preFlagScopingSnapshot := `{"flags": {"a": {"key": "a", "enabled": true, "version": 1}}, "users": {}, "groups": {}, "auditEntries": {}, "environments": {}}`
	err := f.Restore(io.NopCloser(strings.NewReader(preFlagScopingSnapshot)))
	if err == nil {
		t.Fatal("expected Restore to reject a snapshot with pre-environment-scoped flags")
	}
}

// TestFSMApplyEnvironmentRejectsDeletingLastEnvironment guards the
// last-environment invariant at the FSM level -- the ultimate source of
// truth, same as the Admin group's protections.
func TestFSMApplyEnvironmentRejectsDeletingLastEnvironment(t *testing.T) {
	f := newFSM()
	f.environments["prod"] = Environment{ID: "prod", Name: "Production", Order: 0, Version: 1}

	resp := f.applyEnvironment(2, command{Op: opDelete, Entity: entityEnvironment, Key: "prod"})
	respErr, ok := resp.(error)
	if !ok {
		t.Fatalf("expected applyEnvironment to return an error deleting the last environment, got %+v", resp)
	}
	if !errors.Is(respErr, ErrLastEnvironment) {
		t.Fatalf("expected error to be ErrLastEnvironment, got %v", respErr)
	}
	if _, exists := f.environments["prod"]; !exists {
		t.Fatal("expected the last environment to not be deleted")
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
	if f.flags == nil || f.users == nil || f.groups == nil || f.auditEntries == nil || f.environments == nil {
		t.Fatalf("expected all maps to default to empty, got flags=%v users=%v groups=%v auditEntries=%v environments=%v", f.flags, f.users, f.groups, f.auditEntries, f.environments)
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

func TestFSMApplyFlagRejectsUnknownEnvironment(t *testing.T) {
	f := newFSM()

	resp := f.applyFlag(1, command{Op: opSet, Entity: entityFlag, Flag: &Flag{EnvironmentID: "does-not-exist", Key: "checkout", Enabled: true}})
	respErr, ok := resp.(error)
	if !ok {
		t.Fatalf("expected applyFlag to return an error for an unknown environment, got %+v", resp)
	}
	if !errors.Is(respErr, ErrUnknownEnvironment) {
		t.Fatalf("expected error to be ErrUnknownEnvironment, got %v", respErr)
	}
}

func TestFSMApplyFlagSetBatchRejectsUnknownEnvironmentWithNoPartialWrites(t *testing.T) {
	f := newFSM()
	f.environments["a"] = Environment{ID: "a", Name: "A"}
	f.environments["b"] = Environment{ID: "b", Name: "B"}

	resp := f.applyFlag(1, command{Op: opSetBatch, Entity: entityFlag, Flags: []Flag{
		{EnvironmentID: "a", Key: "checkout", Enabled: true},
		{EnvironmentID: "does-not-exist", Key: "checkout", Enabled: true},
		{EnvironmentID: "b", Key: "checkout", Enabled: true},
	}})
	respErr, ok := resp.(error)
	if !ok {
		t.Fatalf("expected applyFlag to return an error for an unknown environment in the batch, got %+v", resp)
	}
	if !errors.Is(respErr, ErrUnknownEnvironment) {
		t.Fatalf("expected error to be ErrUnknownEnvironment, got %v", respErr)
	}
	if len(f.flags) != 0 {
		t.Fatalf("expected zero partial writes when the batch is rejected, got %+v", f.flags)
	}
}

func TestFSMApplyEnvironmentRejectsDeletingEnvironmentWithFlags(t *testing.T) {
	f := newFSM()
	f.environments["prod"] = Environment{ID: "prod", Name: "Production"}
	f.environments["staging"] = Environment{ID: "staging", Name: "Staging"}
	f.flags[flagMapKey("prod", "checkout")] = Flag{EnvironmentID: "prod", Key: "checkout", Enabled: true}

	resp := f.applyEnvironment(3, command{Op: opDelete, Entity: entityEnvironment, Key: "prod"})
	respErr, ok := resp.(error)
	if !ok {
		t.Fatalf("expected applyEnvironment to return an error deleting an environment with flags, got %+v", resp)
	}
	if !errors.Is(respErr, ErrEnvironmentHasFlags) {
		t.Fatalf("expected error to be ErrEnvironmentHasFlags, got %v", respErr)
	}
	if _, exists := f.environments["prod"]; !exists {
		t.Fatal("expected the environment to not be deleted")
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
