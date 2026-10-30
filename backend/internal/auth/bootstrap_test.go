package auth

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"aerendil/backend/internal/store"
)

func TestSeedAdminGroupAndUser_FirstRun(t *testing.T) {
	s := store.NewTestStore(t)
	cfg := AdminConfig{
		Username: "admin",
		Password: "password123",
	}

	if err := SeedAdminGroupAndUser(s, cfg); err != nil {
		t.Fatalf("unexpected error on first seed: %v", err)
	}

	// Verify Admin group
	grp, ok := s.Groups().Get(store.AdminGroupID)
	if !ok {
		t.Fatal("expected Admin group to be seeded")
	}
	if grp.Name != "Admin" || !grp.System {
		t.Fatalf("expected Admin group Name='Admin' System=true, got %+v", grp)
	}

	// Verify admin user
	user, ok := s.Users().GetByUsername("admin")
	if !ok {
		t.Fatal("expected admin user to be seeded")
	}
	if !user.Active {
		t.Fatal("expected admin user to be active")
	}
	if !slices.Contains(user.GroupIDs, store.AdminGroupID) {
		t.Fatalf("expected user to belong to Admin group, got %v", user.GroupIDs)
	}
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte("password123")); err != nil {
		t.Fatalf("password hash does not match seeded password: %v", err)
	}
}

func TestSeedAdminGroupAndUser_SecondRunNoOp(t *testing.T) {
	s := store.NewTestStore(t)
	cfg := AdminConfig{
		Username: "admin",
		Password: "password123",
	}

	if err := SeedAdminGroupAndUser(s, cfg); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}

	userBefore, ok := s.Users().GetByUsername("admin")
	if !ok {
		t.Fatal("expected admin user after first seed")
	}

	// Second run with a different password
	cfgSecond := AdminConfig{
		Username: "admin",
		Password: "differentpassword",
	}
	if err := SeedAdminGroupAndUser(s, cfgSecond); err != nil {
		t.Fatalf("second seed failed: %v", err)
	}

	userAfter, ok := s.Users().GetByUsername("admin")
	if !ok {
		t.Fatal("expected admin user after second seed")
	}

	if userBefore.ID != userAfter.ID {
		t.Fatalf("user ID changed between runs: before=%q after=%q", userBefore.ID, userAfter.ID)
	}
	if string(userBefore.PasswordHash) != string(userAfter.PasswordHash) {
		t.Fatal("password hash was modified on second run; should be a no-op")
	}
	if len(s.Users().List()) != 1 {
		t.Fatalf("expected exactly 1 user, got %d", len(s.Users().List()))
	}
}

func TestSeedAdminGroupAndUser_UsernameCaseInsensitiveAndTrimmed(t *testing.T) {
	s := store.NewTestStore(t)
	cfg := AdminConfig{
		Username: "admin",
		Password: "password123",
	}

	if err := SeedAdminGroupAndUser(s, cfg); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}

	// Seed with differently cased and padded username
	cfgPadded := AdminConfig{
		Username: "  Admin  ",
		Password: "newpassword",
	}
	if err := SeedAdminGroupAndUser(s, cfgPadded); err != nil {
		t.Fatalf("seed with padded/cased username failed: %v", err)
	}

	users := s.Users().List()
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d (seeded duplicate despite casing/padding)", len(users))
	}
	if users[0].Username != "admin" {
		t.Fatalf("expected stored username to be 'admin', got %q", users[0].Username)
	}
}

func TestSeedAdminGroupAndUser_ChangedUsernameWarnsAndSeedsSecondAdmin(t *testing.T) {
	s := store.NewTestStore(t)
	cfg1 := AdminConfig{
		Username: "admin",
		Password: "password123",
	}
	if err := SeedAdminGroupAndUser(s, cfg1); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}

	// Changing username seeds a second admin while first is active
	cfg2 := AdminConfig{
		Username: "secondadmin",
		Password: "password456",
	}
	if err := SeedAdminGroupAndUser(s, cfg2); err != nil {
		t.Fatalf("second seed failed: %v", err)
	}

	if len(s.Users().List()) != 2 {
		t.Fatalf("expected 2 users, got %d", len(s.Users().List()))
	}
	u1, ok1 := s.Users().GetByUsername("admin")
	u2, ok2 := s.Users().GetByUsername("secondadmin")
	if !ok1 || !ok2 {
		t.Fatalf("expected both admin users to exist: ok1=%v, ok2=%v", ok1, ok2)
	}
	if !u1.Active || !u2.Active {
		t.Fatal("expected both admin users to be active")
	}
	if !hasOtherActiveAdmin(s) {
		t.Fatal("expected hasOtherActiveAdmin to return true when multiple active admins exist")
	}
}

func TestSeedAdminGroupAndUser_InactiveExistingAdminDoesNotCount(t *testing.T) {
	s := store.NewTestStore(t)

	// Pre-create the Admin group and an INACTIVE admin user
	if _, err := s.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("create admin group: %v", err)
	}

	inactiveAdmin := store.User{
		ID:           store.NewID(),
		Username:     "oldadmin",
		PasswordHash: []byte("hash"),
		GroupIDs:     []string{store.AdminGroupID},
		Active:       false,
	}
	if _, err := s.Users().Set(inactiveAdmin); err != nil {
		t.Fatalf("create inactive admin: %v", err)
	}

	if hasOtherActiveAdmin(s) {
		t.Fatal("expected hasOtherActiveAdmin to be false when the only admin user is inactive")
	}

	// Seed a new admin
	cfg := AdminConfig{
		Username: "newadmin",
		Password: "password123",
	}
	if err := SeedAdminGroupAndUser(s, cfg); err != nil {
		t.Fatalf("seed new admin: %v", err)
	}

	newAdmin, ok := s.Users().GetByUsername("newadmin")
	if !ok || !newAdmin.Active {
		t.Fatal("expected new admin to be active")
	}
}

func TestSeedAdminGroupAndUser_ExistingAdminGroupNotRecreated(t *testing.T) {
	s := store.NewTestStore(t)

	// Pre-create Admin group with customized system settings or name
	customGroup := store.Group{
		ID:     store.AdminGroupID,
		Name:   "Admin",
		System: true,
	}
	if _, err := s.Groups().Set(customGroup); err != nil {
		t.Fatalf("pre-create admin group: %v", err)
	}

	cfg := AdminConfig{
		Username: "admin",
		Password: "password123",
	}
	if err := SeedAdminGroupAndUser(s, cfg); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	grp, ok := s.Groups().Get(store.AdminGroupID)
	if !ok {
		t.Fatal("admin group disappeared")
	}
	if grp.ID != store.AdminGroupID {
		t.Fatalf("expected group ID %q, got %q", store.AdminGroupID, grp.ID)
	}
}

func TestSeedAdminGroupAndUser_ErrNotLeaderRetriesAndGivesUp(t *testing.T) {
	// Spin up a store with Bootstrap: false (never becomes Raft leader)
	dataDir, err := os.MkdirTemp("", "aerendil-follower-store-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dataDir)
	})

	s, err := store.Open(store.Config{
		NodeID:    "follower-node",
		BindAddr:  "127.0.0.1:0",
		DataDir:   dataDir,
		Bootstrap: false,
	})
	if err != nil {
		t.Fatalf("open non-leader store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	// Override timeout to small duration for fast test
	origTimeout := seedLeaderWaitTimeout
	seedLeaderWaitTimeout = 50 * time.Millisecond
	defer func() {
		seedLeaderWaitTimeout = origTimeout
	}()

	cfg := AdminConfig{
		Username: "admin",
		Password: "password123",
	}

	// Should retry until timeout, log giving up, and return nil
	start := time.Now()
	err = SeedAdminGroupAndUser(s, cfg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error on leader-wait give up, got %v", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("expected to wait at least ~50ms, elapsed: %v", elapsed)
	}

	// Verify nothing was seeded on the follower
	if _, ok := s.Groups().Get(store.AdminGroupID); ok {
		t.Fatal("expected no Admin group to be seeded on follower")
	}
	if _, ok := s.Users().GetByUsername("admin"); ok {
		t.Fatal("expected no admin user to be seeded on follower")
	}
}

func TestSeedAdminGroupAndUser_PasswordTooLongReturnsError(t *testing.T) {
	s := store.NewTestStore(t)
	cfg := AdminConfig{
		Username: "admin",
		Password: strings.Repeat("a", 100),
	}
	err := SeedAdminGroupAndUser(s, cfg)
	if err == nil {
		t.Fatal("expected error for oversized password")
	}
}

