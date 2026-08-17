package auth

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"aerendil/backend/internal/store"
)

const seedLeaderWaitTimeout = 10 * time.Second

// SeedAdminGroupAndUser ensures the protected Admin group and a
// corresponding admin user (from cfg) exist. It's idempotent -- it never
// recreates the group or resets an existing admin's password.
//
// A freshly bootstrapped node isn't leader the instant store.Open returns,
// so this retries on store.ErrNotLeader for a bounded window; a follower
// that never becomes leader just logs and returns nil, since the leader
// will have already seeded the state, which then replicates here.
func SeedAdminGroupAndUser(s *store.Store, cfg AdminConfig) error {
	deadline := time.Now().Add(seedLeaderWaitTimeout)
	for {
		err := seedAdminGroupAndUserOnce(s, cfg)
		if err == nil || !errors.Is(err, store.ErrNotLeader) {
			return err
		}
		if time.Now().After(deadline) {
			log.Printf("auth: giving up waiting to become raft leader to seed admin account: %v", err)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func seedAdminGroupAndUserOnce(s *store.Store, cfg AdminConfig) error {
	if _, ok := s.Groups().Get(store.AdminGroupID); !ok {
		if _, err := s.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
			return fmt.Errorf("seed admin group: %w", err)
		}
		log.Println("auth: seeded Admin group")
	}

	// Usernames are stored lowercase (see Service.Authenticate), so
	// AERENDIL_ADMIN_USERNAME="Admin" matches an existing "admin" instead of
	// seeding a duplicate.
	username := strings.ToLower(strings.TrimSpace(cfg.Username))

	if _, ok := s.Users().GetByUsername(username); ok {
		return nil
	}

	// Changing AERENDIL_ADMIN_USERNAME doesn't rename the existing admin --
	// it seeds a second one. Warn, since the operator likely intended a
	// rename and the original account remains active.
	if hasOtherActiveAdmin(s) {
		log.Printf("auth: AERENDIL_ADMIN_USERNAME is %q but at least one other Admin-group account already exists; seeding a new admin account rather than renaming the existing one -- the original account remains active and must be deactivated/removed manually if that wasn't intended", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	admin := store.User{
		ID:           store.NewID(),
		Username:     username,
		PasswordHash: hash,
		GroupIDs:     []string{store.AdminGroupID},
		Active:       true,
	}
	if _, err := s.Users().Set(admin); err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Printf("auth: seeded admin user %q", username)
	return nil
}

// hasOtherActiveAdmin reports whether any active user already belongs to
// the Admin group, to detect a likely-unintended second admin account.
func hasOtherActiveAdmin(s *store.Store) bool {
	for _, u := range s.Users().List() {
		if u.Active && slices.Contains(u.GroupIDs, store.AdminGroupID) {
			return true
		}
	}
	return false
}
