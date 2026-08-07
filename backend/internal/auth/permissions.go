package auth

import (
	"slices"
	"sort"

	"aerendil/backend/internal/store"
)

const (
	PermFlagsRead          = "flags:read"
	PermFlagsWrite         = "flags:write"
	PermUsersRead          = "users:read"
	PermUsersCreate        = "users:create"
	PermUsersUpdate        = "users:update"
	PermUsersDelete        = "users:delete"
	PermGroupsRead         = "groups:read"
	PermGroupsCreate       = "groups:create"
	PermGroupsUpdate       = "groups:update"
	PermGroupsDelete       = "groups:delete"
	PermAuditsRead         = "audits:read"
	PermEnvironmentsRead   = "environments:read"
	PermEnvironmentsCreate = "environments:create"
	PermEnvironmentsUpdate = "environments:update"
	PermEnvironmentsDelete = "environments:delete"

	PermApplicationCredentialsRead   = "applicationCredentials:read"
	PermApplicationCredentialsCreate = "applicationCredentials:create"
	PermApplicationCredentialsUpdate = "applicationCredentials:update"
	PermApplicationCredentialsDelete = "applicationCredentials:delete"
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
	PermEnvironmentsRead, PermEnvironmentsCreate, PermEnvironmentsUpdate, PermEnvironmentsDelete,
	PermApplicationCredentialsRead, PermApplicationCredentialsCreate,
	PermApplicationCredentialsUpdate, PermApplicationCredentialsDelete,
}

// IsKnownPermission reports whether perm is one of AllPermissions. This
// gates what a human Group can be granted (including the
// applicationCredentials:* permissions themselves, i.e. who may manage
// application credentials) -- it is deliberately broader than
// CredentialScopes below, which gates what an application credential's own
// Scopes may contain.
func IsKnownPermission(perm string) bool {
	return slices.Contains(AllPermissions, perm)
}

// CredentialScopes is the restricted catalog an application credential's
// own Scopes field is validated against (see
// api.applicationCredentialsPostHandler/PutHandler) -- deliberately just
// flags:read/flags:write, not the full AllPermissions catalog, so a
// compromised or over-granted application credential can never manage
// users, groups, environments, or other credentials.
var CredentialScopes = []string{PermFlagsRead, PermFlagsWrite}

// IsKnownCredentialScope reports whether scope is one of CredentialScopes.
func IsKnownCredentialScope(scope string) bool {
	return slices.Contains(CredentialScopes, scope)
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

// EnvironmentSet is a resolved, deduplicated set of environment IDs a
// principal has been granted access to via their groups' EnvironmentIDs.
// Structurally identical to PermissionSet -- kept as a distinct type so a
// caller can't accidentally pass one where the other is expected.
type EnvironmentSet map[string]struct{}

// Has reports whether environmentID is in the set.
func (e EnvironmentSet) Has(environmentID string) bool {
	_, ok := e[environmentID]
	return ok
}

// Keys returns the environment IDs in the set, sorted for stable
// JSON/test output.
func (e EnvironmentSet) Keys() []string {
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Resolve computes the effective, live permission and environment-access
// sets for a user: the Admin group short-circuits to "all access"
// (isAdmin=true); otherwise perms/envs are the union of all
// permissions/EnvironmentIDs across the user's groups. This is called fresh
// on every permission-gated request -- no caching, nothing baked into the
// JWT -- so group membership/permission changes and deactivation take
// effect on the user's very next request.
func (s *Service) Resolve(userID string) (perms PermissionSet, envs EnvironmentSet, isAdmin bool, err error) {
	u, ok := s.store.Users().Get(userID)
	if !ok || !u.Active {
		return nil, nil, false, ErrUserNotFound
	}

	perms, envs, isAdmin = s.resolvePermissionsForUser(u)
	return perms, envs, isAdmin, nil
}

// resolvePermissionsForUser computes the effective permission and
// environment-access sets for an already-fetched user record, without an
// additional store lookup -- the shared logic behind both Resolve and
// AuthenticateToken (service.go), so a call site that already has the user
// in hand never re-fetches it.
func (s *Service) resolvePermissionsForUser(u store.User) (perms PermissionSet, envs EnvironmentSet, isAdmin bool) {
	perms = PermissionSet{}
	envs = EnvironmentSet{}
	for _, groupID := range u.GroupIDs {
		g, ok := s.store.Groups().Get(groupID)
		if !ok {
			continue
		}
		if g.ID == store.AdminGroupID && g.System {
			return nil, nil, true
		}
		for _, perm := range g.Permissions {
			perms[perm] = struct{}{}
		}
		for _, envID := range g.EnvironmentIDs {
			envs[envID] = struct{}{}
		}
	}
	return perms, envs, false
}
