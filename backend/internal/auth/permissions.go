package auth

import (
	"slices"
	"sort"

	"aerendil/backend/internal/store"
)

// Perm* are the individual permission strings ("resource:action"). See
// AllPermissions for the full catalog and split rationale.
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
// surfaced to the Groups UI's permission picker and used to reject
// unknown/typo'd strings in incoming group.Permissions.
//
// Users and Groups are split into per-action read/create/update/delete
// permissions (unlike Flags' single "write") so a group can be granted
// users:create without also getting users:delete.
var AllPermissions = []string{
	PermFlagsRead, PermFlagsWrite,
	PermUsersRead, PermUsersCreate, PermUsersUpdate, PermUsersDelete,
	PermGroupsRead, PermGroupsCreate, PermGroupsUpdate, PermGroupsDelete,
	PermAuditsRead,
	PermEnvironmentsRead, PermEnvironmentsCreate, PermEnvironmentsUpdate, PermEnvironmentsDelete,
	PermApplicationCredentialsRead, PermApplicationCredentialsCreate,
	PermApplicationCredentialsUpdate, PermApplicationCredentialsDelete,
}

// IsKnownPermission reports whether perm is one of AllPermissions -- gates
// what a human Group can be granted, deliberately broader than
// CredentialScopes below.
func IsKnownPermission(perm string) bool {
	return slices.Contains(AllPermissions, perm)
}

// CredentialScopes is the restricted catalog an application credential's own
// Scopes are validated against -- just flags:read/write, so a compromised
// credential can never manage users, groups, or other credentials.
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

// EnvironmentSet is a resolved set of environment IDs a principal has
// access to via their groups. Structurally identical to PermissionSet, kept
// distinct so a caller can't pass one where the other is expected.
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

// Resolve computes the live permission and environment-access sets for a
// user: Admin group membership short-circuits to "all access"; otherwise
// perms/envs are the union across the user's groups. Called fresh on every
// request, so changes take effect immediately.
func (s *Service) Resolve(userID string) (perms PermissionSet, envs EnvironmentSet, isAdmin bool, err error) {
	u, ok := s.store.Users().Get(userID)
	if !ok || !u.Active {
		return nil, nil, false, ErrUserNotFound
	}

	perms, envs, isAdmin = s.resolvePermissionsForUser(u)
	return perms, envs, isAdmin, nil
}

// resolvePermissionsForUser is the shared logic behind Resolve and
// AuthenticateToken, for a call site that already has the user record.
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
