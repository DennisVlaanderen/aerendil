package auth

import (
	"errors"
	"sort"

	"aerendil/backend/internal/store"
)

const (
	PermFlagsRead    = "flags:read"
	PermFlagsWrite   = "flags:write"
	PermUsersRead    = "users:read"
	PermUsersCreate  = "users:create"
	PermUsersUpdate  = "users:update"
	PermUsersDelete  = "users:delete"
	PermGroupsRead   = "groups:read"
	PermGroupsCreate = "groups:create"
	PermGroupsUpdate = "groups:update"
	PermGroupsDelete = "groups:delete"
	PermAuditsRead   = "audits:read"
)

// AllPermissions is the catalog of every known permission string --
// surfaced to the Groups UI for building a permission picker, and used to
// validate incoming group.Permissions so a client can't smuggle in an
// unknown or typo'd string. Gating a new endpoint elsewhere is "add one
// const here, reference it in one route registration line" -- nothing else
// in the permission model needs to change.
//
// Users and Groups are split into per-action read/create/update/delete
// permissions rather than a single coarse "write" (unlike Flags, which has
// no separate create/update/delete endpoints) so a group can be granted,
// say, users:create and users:update without also being able to delete
// users.
var AllPermissions = []string{
	PermFlagsRead, PermFlagsWrite,
	PermUsersRead, PermUsersCreate, PermUsersUpdate, PermUsersDelete,
	PermGroupsRead, PermGroupsCreate, PermGroupsUpdate, PermGroupsDelete,
	PermAuditsRead,
}

// IsKnownPermission reports whether perm is one of AllPermissions.
func IsKnownPermission(perm string) bool {
	for _, p := range AllPermissions {
		if p == perm {
			return true
		}
	}
	return false
}

// PermissionSet is a resolved, deduplicated set of permission strings.
type PermissionSet map[string]struct{}

// Has reports whether perm is in the set.
func (p PermissionSet) Has(perm string) bool {
	_, ok := p[perm]
	return ok
}

// Keys returns the permission strings in the set, sorted for stable
// JSON/test output.
func (p PermissionSet) Keys() []string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Resolve computes the effective, live permission set for a user: the
// Admin group short-circuits to "all access" (isAdmin=true); otherwise
// perms is the union of all permissions across the user's groups. This is
// called fresh on every permission-gated request -- no caching, nothing
// baked into the JWT -- so group membership/permission changes and
// deactivation take effect on the user's very next request.
func (s *Service) Resolve(userID string) (perms PermissionSet, isAdmin bool, err error) {
	u, ok := s.store.Users().Get(userID)
	if !ok || !u.Active {
		return nil, false, errors.New("user not found or inactive")
	}

	perms, isAdmin = s.resolvePermissionsForUser(u)
	return perms, isAdmin, nil
}

// resolvePermissionsForUser computes the effective permission set for an
// already-fetched user record, without an additional store lookup -- the
// shared logic behind both Resolve and AuthenticateToken (service.go), so a
// call site that already has the user in hand never re-fetches it.
func (s *Service) resolvePermissionsForUser(u store.User) (perms PermissionSet, isAdmin bool) {
	perms = PermissionSet{}
	for _, groupID := range u.GroupIDs {
		g, ok := s.store.Groups().Get(groupID)
		if !ok {
			continue
		}
		if g.ID == store.AdminGroupID && g.System {
			return nil, true
		}
		for _, perm := range g.Permissions {
			perms[perm] = struct{}{}
		}
	}
	return perms, false
}
