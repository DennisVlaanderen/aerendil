// A structural (not imported) type -- lib/server/auth's Session satisfies
// this, but this module intentionally doesn't import from lib/server so it
// stays safe to use from client-reachable code (e.g. Sidebar.svelte)
// without tripping SvelteKit's server-only import boundary.
export interface PermissionAware {
	isAdmin: boolean;
	permissions: string[];
}

// The Admin bypass is unconditional, mirroring the backend's
// auth.Service.Resolve: an admin is never blocked by a missing permission
// string, regardless of what a group is or isn't configured with.
export function hasPermission(session: PermissionAware, perm: string): boolean {
	return session.isAdmin || session.permissions.includes(perm);
}

// A separate structural type from PermissionAware (rather than folding
// environmentIds into it) so a call site that only cares about one
// dimension doesn't need to fake the other.
export interface EnvironmentAware {
	isAdmin: boolean;
	environmentIds: string[];
}

// Mirrors hasPermission's admin-bypass shape -- and the backend's
// api.resolvedPrincipal.hasEnvironmentAccess (middleware.go): Admin can
// touch any environment unconditionally, everyone else only what their
// groups grant.
export function hasEnvironmentAccess(session: EnvironmentAware, environmentId: string): boolean {
	return session.isAdmin || session.environmentIds.includes(environmentId);
}
