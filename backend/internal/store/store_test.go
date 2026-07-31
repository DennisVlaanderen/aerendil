package store

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// newTestStore builds a single-node Raft cluster entirely in memory -- no
// disk, no real sockets -- using the same in-memory harness hashicorp/raft's
// own test suite relies on.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	fsmStore := newFSM()
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID("test-node")
	config.HeartbeatTimeout = 50 * time.Millisecond
	config.ElectionTimeout = 50 * time.Millisecond
	config.LeaderLeaseTimeout = 50 * time.Millisecond
	config.CommitTimeout = 5 * time.Millisecond

	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapshotStore := raft.NewInmemSnapshotStore()
	_, transport := raft.NewInmemTransport("")

	r, err := raft.NewRaft(config, fsmStore, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		t.Fatalf("start raft node: %v", err)
	}

	bootstrapConfig := raft.Configuration{
		Servers: []raft.Server{{ID: config.LocalID, Address: transport.LocalAddr()}},
	}
	if err := r.BootstrapCluster(bootstrapConfig).Error(); err != nil {
		t.Fatalf("bootstrap cluster: %v", err)
	}

	s := &Store{raft: r, fsm: fsmStore}
	t.Cleanup(func() {
		_ = s.Close()
	})

	waitForLeader(t, s)
	return s
}

func waitForLeader(t *testing.T, s *Store) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.raft.State() == raft.Leader {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for raft node to become leader")
}

// seedEnvironment creates and returns an Environment's ID for tests that
// need one to scope flags to.
func seedEnvironment(t *testing.T, s *Store, name string) string {
	t.Helper()
	env, err := s.Environments().Set(Environment{ID: NewID(), Name: name})
	if err != nil {
		t.Fatalf("seed environment %q: %v", name, err)
	}
	return env.ID
}

func TestStoreSetAndGet(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	applied, err := s.Flags().Set(Flag{EnvironmentID: envID, Key: "new-checkout", Enabled: true, Value: "on"})
	if err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}
	if applied.Version == 0 {
		t.Fatal("expected applied flag to have a non-zero version")
	}

	got, ok := s.Flags().Get(envID, "new-checkout")
	if !ok {
		t.Fatal("expected flag to be present after set")
	}
	if !got.Enabled || got.Value != "on" {
		t.Fatalf("unexpected flag state: %+v", got)
	}

	flags := s.Flags().List(envID)
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
}

func TestStoreSetOverwritesExistingFlag(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	if _, err := s.Flags().Set(Flag{EnvironmentID: envID, Key: "checkout", Enabled: true}); err != nil {
		t.Fatalf("expected first set to succeed: %v", err)
	}
	if _, err := s.Flags().Set(Flag{EnvironmentID: envID, Key: "checkout", Enabled: false}); err != nil {
		t.Fatalf("expected second set to succeed: %v", err)
	}

	got, ok := s.Flags().Get(envID, "checkout")
	if !ok {
		t.Fatal("expected flag to still be present")
	}
	if got.Enabled {
		t.Fatal("expected the later Set to win")
	}
}

func TestStoreSetFlagRejectsUnknownEnvironment(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Flags().Set(Flag{EnvironmentID: "does-not-exist", Key: "checkout", Enabled: true}); !errors.Is(err, ErrUnknownEnvironment) {
		t.Fatalf("expected ErrUnknownEnvironment, got %v", err)
	}
}

func TestStoreFlagsAreIsolatedPerEnvironment(t *testing.T) {
	s := newTestStore(t)
	prodID := seedEnvironment(t, s, "Production")
	stagingID := seedEnvironment(t, s, "Staging")

	if _, err := s.Flags().Set(Flag{EnvironmentID: prodID, Key: "checkout", Enabled: true}); err != nil {
		t.Fatalf("expected set in prod to succeed: %v", err)
	}
	if _, err := s.Flags().Set(Flag{EnvironmentID: stagingID, Key: "checkout", Enabled: false}); err != nil {
		t.Fatalf("expected set in staging to succeed: %v", err)
	}

	prodFlag, ok := s.Flags().Get(prodID, "checkout")
	if !ok || !prodFlag.Enabled {
		t.Fatalf("expected prod's checkout to be enabled independently, got %+v (ok=%v)", prodFlag, ok)
	}
	stagingFlag, ok := s.Flags().Get(stagingID, "checkout")
	if !ok || stagingFlag.Enabled {
		t.Fatalf("expected staging's checkout to be disabled independently, got %+v (ok=%v)", stagingFlag, ok)
	}

	if len(s.Flags().List(prodID)) != 1 || len(s.Flags().List(stagingID)) != 1 {
		t.Fatalf("expected each environment to list exactly its own flag, got prod=%+v staging=%+v", s.Flags().List(prodID), s.Flags().List(stagingID))
	}
}

func TestStoreSetManyWritesAcrossEnvironmentsAtomically(t *testing.T) {
	s := newTestStore(t)
	envA := seedEnvironment(t, s, "A")
	envB := seedEnvironment(t, s, "B")
	envC := seedEnvironment(t, s, "C")

	applied, err := s.Flags().SetMany("checkout", true, "on", []string{envA, envB, envC})
	if err != nil {
		t.Fatalf("expected SetMany to succeed: %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("expected 3 applied flags, got %d", len(applied))
	}
	version := applied[0].Version
	for _, f := range applied {
		if f.Version != version {
			t.Fatalf("expected every flag in the batch to share the same Version, got %+v", applied)
		}
	}

	for _, envID := range []string{envA, envB, envC} {
		got, ok := s.Flags().Get(envID, "checkout")
		if !ok || !got.Enabled || got.Value != "on" {
			t.Fatalf("expected checkout to exist in environment %q, got %+v (ok=%v)", envID, got, ok)
		}
	}
}

func TestStoreSetManyRejectsUnknownEnvironmentWithNoPartialWrites(t *testing.T) {
	s := newTestStore(t)
	envA := seedEnvironment(t, s, "A")
	envB := seedEnvironment(t, s, "B")

	_, err := s.Flags().SetMany("checkout", true, "on", []string{envA, "does-not-exist", envB})
	if !errors.Is(err, ErrUnknownEnvironment) {
		t.Fatalf("expected ErrUnknownEnvironment, got %v", err)
	}

	if _, ok := s.Flags().Get(envA, "checkout"); ok {
		t.Fatal("expected no partial write to envA when the batch is rejected")
	}
	if _, ok := s.Flags().Get(envB, "checkout"); ok {
		t.Fatal("expected no partial write to envB when the batch is rejected")
	}
}

func TestStoreDeleteEnvironmentRejectsWhenFlagsStillReferenceIt(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")
	// A second environment exists so this test isolates ErrEnvironmentHasFlags
	// from ErrLastEnvironment.
	seedEnvironment(t, s, "Staging")

	if _, err := s.Flags().Set(Flag{EnvironmentID: envID, Key: "checkout", Enabled: true}); err != nil {
		t.Fatalf("expected set to succeed: %v", err)
	}

	if err := s.Environments().Delete(envID); !errors.Is(err, ErrEnvironmentHasFlags) {
		t.Fatalf("expected ErrEnvironmentHasFlags, got %v", err)
	}
	if _, ok := s.Environments().Get(envID); !ok {
		t.Fatal("expected the environment to still exist after rejected delete")
	}
}

func TestStoreSetAndGetUser(t *testing.T) {
	s := newTestStore(t)

	applied, err := s.Users().Set(User{ID: "u1", Username: "alice", Active: true, GroupIDs: []string{"editors"}})
	if err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}
	if applied.Version == 0 {
		t.Fatal("expected applied user to have a non-zero version")
	}

	got, ok := s.Users().Get("u1")
	if !ok {
		t.Fatal("expected user to be present after SetUser")
	}
	if got.Username != "alice" || !got.Active {
		t.Fatalf("unexpected user state: %+v", got)
	}

	byUsername, ok := s.Users().GetByUsername("alice")
	if !ok || byUsername.ID != "u1" {
		t.Fatalf("expected GetUserByUsername to find u1, got %+v (ok=%v)", byUsername, ok)
	}

	if len(s.Users().List()) != 1 {
		t.Fatalf("expected 1 user, got %d", len(s.Users().List()))
	}
}

func TestStoreDeleteUser(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Users().Set(User{ID: "u1", Username: "alice", Active: true}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}
	if err := s.Users().Delete("u1"); err != nil {
		t.Fatalf("expected DeleteUser to succeed: %v", err)
	}
	if _, ok := s.Users().Get("u1"); ok {
		t.Fatal("expected user to be gone after DeleteUser")
	}
}

func TestStoreSetUserRejectsDuplicateUsername(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Users().Set(User{ID: "u1", Username: "alice", Active: true}); err != nil {
		t.Fatalf("expected first SetUser to succeed: %v", err)
	}

	if _, err := s.Users().Set(User{ID: "u2", Username: "alice", Active: true}); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken for a duplicate username, got %v", err)
	}

	if len(s.Users().List()) != 1 {
		t.Fatalf("expected only 1 user to exist, got %d", len(s.Users().List()))
	}
}

func TestStoreDeleteUserRejectsSoleActiveAdmin(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("seed admin group: %v", err)
	}
	if _, err := s.Users().Set(User{ID: "u1", Username: "sole-admin", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}

	if err := s.Users().Delete("u1"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin deleting the sole active admin, got %v", err)
	}
	if _, ok := s.Users().Get("u1"); !ok {
		t.Fatal("expected sole admin to still exist after rejected delete")
	}
}

func TestStoreDeleteUserAllowsAdminWhenAnotherAdminRemains(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("seed admin group: %v", err)
	}
	if _, err := s.Users().Set(User{ID: "u1", Username: "admin-one", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}
	if _, err := s.Users().Set(User{ID: "u2", Username: "admin-two", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}

	if err := s.Users().Delete("u1"); err != nil {
		t.Fatalf("expected deleting one of two admins to succeed: %v", err)
	}
	if _, ok := s.Users().Get("u1"); ok {
		t.Fatal("expected u1 to be gone after delete")
	}
}

func TestStoreSetUserRejectsDeactivatingSoleActiveAdmin(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("seed admin group: %v", err)
	}
	if _, err := s.Users().Set(User{ID: "u1", Username: "sole-admin", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}

	if _, err := s.Users().Set(User{ID: "u1", Username: "sole-admin", Active: false, GroupIDs: []string{AdminGroupID}}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin deactivating the sole active admin, got %v", err)
	}

	got, ok := s.Users().Get("u1")
	if !ok || !got.Active {
		t.Fatalf("expected sole admin to remain active after rejected update, got %+v (ok=%v)", got, ok)
	}
}

func TestStoreSetUserRejectsRemovingSoleActiveAdminFromAdminGroup(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("seed admin group: %v", err)
	}
	if _, err := s.Users().Set(User{ID: "u1", Username: "sole-admin", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}

	if _, err := s.Users().Set(User{ID: "u1", Username: "sole-admin", Active: true, GroupIDs: []string{}}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin stripping the sole active admin's Admin group membership, got %v", err)
	}

	got, ok := s.Users().Get("u1")
	if !ok || !slices.Contains(got.GroupIDs, AdminGroupID) {
		t.Fatalf("expected sole admin to keep Admin group membership after rejected update, got %+v (ok=%v)", got, ok)
	}
}

func TestStoreSetUserAllowsUnrelatedEditsToSoleActiveAdmin(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("seed admin group: %v", err)
	}
	if _, err := s.Users().Set(User{ID: "u1", Username: "sole-admin", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected SetUser to succeed: %v", err)
	}

	if _, err := s.Users().Set(User{ID: "u1", Username: "renamed-admin", Active: true, GroupIDs: []string{AdminGroupID}}); err != nil {
		t.Fatalf("expected renaming the sole active admin (while remaining an active admin) to succeed: %v", err)
	}

	got, ok := s.Users().Get("u1")
	if !ok || got.Username != "renamed-admin" {
		t.Fatalf("expected sole admin's username to be updated, got %+v (ok=%v)", got, ok)
	}
}

func TestStoreSetAndGetGroup(t *testing.T) {
	s := newTestStore(t)

	applied, err := s.Groups().Set(Group{ID: "editors", Name: "Editors", Permissions: []string{"flags:read", "flags:write"}})
	if err != nil {
		t.Fatalf("expected SetGroup to succeed: %v", err)
	}
	if applied.Version == 0 {
		t.Fatal("expected applied group to have a non-zero version")
	}

	got, ok := s.Groups().Get("editors")
	if !ok {
		t.Fatal("expected group to be present after SetGroup")
	}
	if got.Name != "Editors" || len(got.Permissions) != 2 {
		t.Fatalf("unexpected group state: %+v", got)
	}

	if len(s.Groups().List()) != 1 {
		t.Fatalf("expected 1 group, got %d", len(s.Groups().List()))
	}
}

func TestStoreSetGroupRejectsSystemGroupEdit(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("expected initial seed of the Admin group to succeed: %v", err)
	}

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Renamed"}); err == nil {
		t.Fatal("expected editing the Admin group to be rejected")
	}

	got, ok := s.Groups().Get(AdminGroupID)
	if !ok || got.Name != "Admin" {
		t.Fatalf("expected Admin group to be unchanged, got %+v (ok=%v)", got, ok)
	}
}

func TestStoreDeleteGroupRejectsSystemGroup(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("expected initial seed of the Admin group to succeed: %v", err)
	}

	if err := s.Groups().Delete(AdminGroupID); err == nil {
		t.Fatal("expected deleting the Admin group to be rejected")
	}

	if _, ok := s.Groups().Get(AdminGroupID); !ok {
		t.Fatal("expected Admin group to still exist")
	}
}

func TestStoreDeleteGroupAllowsNonSystemGroup(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Groups().Set(Group{ID: "editors", Name: "Editors"}); err != nil {
		t.Fatalf("expected SetGroup to succeed: %v", err)
	}
	if err := s.Groups().Delete("editors"); err != nil {
		t.Fatalf("expected DeleteGroup to succeed for a non-system group: %v", err)
	}
	if _, ok := s.Groups().Get("editors"); ok {
		t.Fatal("expected group to be gone after DeleteGroup")
	}
}

func TestStoreAuditsAppendAndList(t *testing.T) {
	s := newTestStore(t)

	first, err := s.Audits().Append(AuditEntry{ActorID: "u1", Action: "flag.set", TargetType: "flag", TargetID: "checkout", Success: true, StatusCode: 200})
	if err != nil {
		t.Fatalf("expected Append to succeed: %v", err)
	}
	if first.ID == 0 {
		t.Fatal("expected appended entry to have a non-zero ID")
	}
	if first.Timestamp == 0 {
		t.Fatal("expected Append to assign a non-zero Timestamp")
	}

	second, err := s.Audits().Append(AuditEntry{ActorID: "u2", Action: "user.create", TargetType: "user", TargetID: "u3", Success: false, StatusCode: 409, Error: "username is already taken"})
	if err != nil {
		t.Fatalf("expected second Append to succeed: %v", err)
	}

	got, ok := s.Audits().Get(first.ID)
	if !ok || got.Action != "flag.set" {
		t.Fatalf("expected Get to find the first entry, got %+v (ok=%v)", got, ok)
	}

	all := s.Audits().List(AuditFilter{})
	if len(all) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(all))
	}
	if all[0].ID != second.ID {
		t.Fatalf("expected List to order newest first, got %+v", all)
	}

	byTargetType := s.Audits().List(AuditFilter{TargetType: "user"})
	if len(byTargetType) != 1 || byTargetType[0].TargetID != "u3" {
		t.Fatalf("expected 1 entry filtered by TargetType=user, got %+v", byTargetType)
	}

	byTargetID := s.Audits().List(AuditFilter{TargetID: "checkout"})
	if len(byTargetID) != 1 || byTargetID[0].ActorID != "u1" {
		t.Fatalf("expected 1 entry filtered by TargetID=checkout, got %+v", byTargetID)
	}

	byActor := s.Audits().List(AuditFilter{ActorID: "u2"})
	if len(byActor) != 1 || byActor[0].TargetID != "u3" {
		t.Fatalf("expected 1 entry filtered by ActorID=u2, got %+v", byActor)
	}

	combined := s.Audits().List(AuditFilter{TargetType: "flag", ActorID: "u2"})
	if len(combined) != 0 {
		t.Fatalf("expected AND-combined filter to match nothing, got %+v", combined)
	}
}

func TestStoreSetAndGetEnvironment(t *testing.T) {
	s := newTestStore(t)

	applied, err := s.Environments().Set(Environment{ID: "prod", Name: "Production"})
	if err != nil {
		t.Fatalf("expected SetEnvironment to succeed: %v", err)
	}
	if applied.Version == 0 {
		t.Fatal("expected applied environment to have a non-zero version")
	}

	got, ok := s.Environments().Get("prod")
	if !ok {
		t.Fatal("expected environment to be present after SetEnvironment")
	}
	if got.Name != "Production" {
		t.Fatalf("unexpected environment state: %+v", got)
	}

	if len(s.Environments().List()) != 1 {
		t.Fatalf("expected 1 environment, got %d", len(s.Environments().List()))
	}
}

func TestStoreListEnvironmentsReturnsStableOrder(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Environments().Set(Environment{ID: "c", Name: "Third", Order: 2}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}
	if _, err := s.Environments().Set(Environment{ID: "a", Name: "First", Order: 0}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}
	if _, err := s.Environments().Set(Environment{ID: "b", Name: "Second", Order: 1}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}

	got := s.Environments().List()
	if len(got) != 3 {
		t.Fatalf("expected 3 environments, got %d", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "c" {
		t.Fatalf("expected List to be ordered by Order ascending, got %+v", got)
	}
}

func TestStoreDeleteEnvironmentAllowsNonLastEnvironment(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Environments().Set(Environment{ID: "prod", Name: "Production", Order: 0}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}
	if _, err := s.Environments().Set(Environment{ID: "staging", Name: "Staging", Order: 1}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}

	if err := s.Environments().Delete("staging"); err != nil {
		t.Fatalf("expected DeleteEnvironment to succeed when another environment remains: %v", err)
	}
	if _, ok := s.Environments().Get("staging"); ok {
		t.Fatal("expected environment to be gone after DeleteEnvironment")
	}
}

func TestStoreDeleteEnvironmentRejectsLastEnvironment(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Environments().Set(Environment{ID: "prod", Name: "Production", Order: 0}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}

	if err := s.Environments().Delete("prod"); !errors.Is(err, ErrLastEnvironment) {
		t.Fatalf("expected ErrLastEnvironment deleting the last remaining environment, got %v", err)
	}
	if _, ok := s.Environments().Get("prod"); !ok {
		t.Fatal("expected the last environment to still exist after rejected delete")
	}
}

type memSnapshotSink struct {
	buf bytes.Buffer
}

func (s *memSnapshotSink) Write(p []byte) (int, error) { return s.buf.Write(p) }
func (s *memSnapshotSink) Close() error                { return nil }
func (s *memSnapshotSink) ID() string                  { return "test-snapshot" }
func (s *memSnapshotSink) Cancel() error               { return nil }

func (s *memSnapshotSink) reader() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(s.buf.Bytes()))
}

func TestFSMSnapshotRestoreRoundTrip(t *testing.T) {
	f := newFSM()
	f.flags[flagMapKey("prod", "a")] = Flag{EnvironmentID: "prod", Key: "a", Enabled: true, Version: 1}
	f.flags[flagMapKey("prod", "b")] = Flag{EnvironmentID: "prod", Key: "b", Enabled: false, Version: 2}

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

	if len(restored.flags) != 2 {
		t.Fatalf("expected 2 restored flags, got %d", len(restored.flags))
	}
	if !restored.flags[flagMapKey("prod", "a")].Enabled || restored.flags[flagMapKey("prod", "b")].Enabled {
		t.Fatalf("unexpected restored state: %+v", restored.flags)
	}
}
