package auth

import (
	"testing"

	"aerendil/backend/internal/store"
)

func TestResolveAdminGroupBypasses(t *testing.T) {
	s := store.NewTestStore(t)
	if _, err := s.Groups().Set(store.Group{ID: store.AdminGroupID, Name: "Admin", System: true}); err != nil {
		t.Fatalf("seed admin group: %v", err)
	}
	user, err := s.Users().Set(store.User{ID: store.NewID(), Username: "root", Active: true, GroupIDs: []string{store.AdminGroupID}})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewService("test-secret", s)
	perms, envs, isAdmin, err := service.Resolve(user.ID)
	if err != nil {
		t.Fatalf("expected resolve to succeed: %v", err)
	}
	if !isAdmin {
		t.Fatal("expected user in Admin group to resolve as admin")
	}
	if len(perms) != 0 {
		t.Fatalf("expected empty perms set for admin bypass, got %v", perms)
	}
	if len(envs) != 0 {
		t.Fatalf("expected empty envs set for admin bypass, got %v", envs)
	}
}

func TestResolveUnionsPermissionsAcrossGroups(t *testing.T) {
	s := store.NewTestStore(t)
	if _, err := s.Groups().Set(store.Group{ID: "editors", Name: "Editors", Permissions: []string{PermFlagsRead, PermFlagsWrite}}); err != nil {
		t.Fatalf("create editors group: %v", err)
	}
	if _, err := s.Groups().Set(store.Group{ID: "user-admins", Name: "User Admins", Permissions: []string{PermUsersRead, PermUsersUpdate}}); err != nil {
		t.Fatalf("create user-admins group: %v", err)
	}
	user, err := s.Users().Set(store.User{ID: store.NewID(), Username: "alice", Active: true, GroupIDs: []string{"editors", "user-admins"}})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewService("test-secret", s)
	perms, _, isAdmin, err := service.Resolve(user.ID)
	if err != nil {
		t.Fatalf("expected resolve to succeed: %v", err)
	}
	if isAdmin {
		t.Fatal("expected non-admin user to not resolve as admin")
	}
	for _, want := range []string{PermFlagsRead, PermFlagsWrite, PermUsersRead, PermUsersUpdate} {
		if !perms.Has(want) {
			t.Fatalf("expected resolved permission set to include %q, got %v", want, perms.Keys())
		}
	}
}

func TestResolveUnionsEnvironmentIDsAcrossGroups(t *testing.T) {
	s := store.NewTestStore(t)
	if _, err := s.Groups().Set(store.Group{ID: "prod-access", Name: "Prod Access", EnvironmentIDs: []string{"prod"}}); err != nil {
		t.Fatalf("create prod-access group: %v", err)
	}
	if _, err := s.Groups().Set(store.Group{ID: "staging-access", Name: "Staging Access", EnvironmentIDs: []string{"staging"}}); err != nil {
		t.Fatalf("create staging-access group: %v", err)
	}
	user, err := s.Users().Set(store.User{ID: store.NewID(), Username: "bob", Active: true, GroupIDs: []string{"prod-access", "staging-access"}})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewService("test-secret", s)
	_, envs, isAdmin, err := service.Resolve(user.ID)
	if err != nil {
		t.Fatalf("expected resolve to succeed: %v", err)
	}
	if isAdmin {
		t.Fatal("expected non-admin user to not resolve as admin")
	}
	for _, want := range []string{"prod", "staging"} {
		if !envs.Has(want) {
			t.Fatalf("expected resolved environment set to include %q, got %v", want, envs.Keys())
		}
	}
}

func TestResolveUserWithNoEnvironmentGrantsHasNoEnvironmentAccess(t *testing.T) {
	s := store.NewTestStore(t)
	if _, err := s.Groups().Set(store.Group{ID: "editors", Name: "Editors", Permissions: []string{PermFlagsRead}}); err != nil {
		t.Fatalf("create editors group: %v", err)
	}
	user, err := s.Users().Set(store.User{ID: store.NewID(), Username: "carol", Active: true, GroupIDs: []string{"editors"}})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewService("test-secret", s)
	_, envs, _, err := service.Resolve(user.ID)
	if err != nil {
		t.Fatalf("expected resolve to succeed: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("expected a group with no EnvironmentIDs to grant no environment access, got %v", envs.Keys())
	}
}

func TestResolveUserWithNoGroupsHasNoPermissions(t *testing.T) {
	s := store.NewTestStore(t)
	user, err := s.Users().Set(store.User{ID: store.NewID(), Username: "nobody", Active: true})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewService("test-secret", s)
	perms, _, isAdmin, err := service.Resolve(user.ID)
	if err != nil {
		t.Fatalf("expected resolve to succeed: %v", err)
	}
	if isAdmin {
		t.Fatal("expected user with no groups to not be admin")
	}
	if len(perms) != 0 {
		t.Fatalf("expected no permissions, got %v", perms.Keys())
	}
}

func TestResolveFailsForDeactivatedUser(t *testing.T) {
	s := store.NewTestStore(t)
	user, err := s.Users().Set(store.User{ID: store.NewID(), Username: "gone", Active: false})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewService("test-secret", s)
	if _, _, _, err := service.Resolve(user.ID); err == nil {
		t.Fatal("expected resolve to fail for a deactivated user")
	}
}

func TestResolveFailsForUnknownUser(t *testing.T) {
	s := store.NewTestStore(t)
	service := NewService("test-secret", s)
	if _, _, _, err := service.Resolve("does-not-exist"); err == nil {
		t.Fatal("expected resolve to fail for an unknown user id")
	}
}
