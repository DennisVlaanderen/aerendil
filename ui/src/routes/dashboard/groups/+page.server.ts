import { error } from '@sveltejs/kit';
import { getAuthToken } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { ALL_PERMISSIONS, listGroups } from '$lib/server/groups';
import { listEnvironments } from '$lib/server/environments';
import type { PageServerLoad } from './$types';

// Auth itself is already enforced by dashboard/+layout.server.ts; this load
// only adds the additional permission narrowing on top, the same layering
// dashboard/flags/[key]/+page.server.ts already uses for its own 404 check.
// Mutations (create/update/delete) live in +server.ts / [id]/+server.ts
// instead of form actions, so their responses carry real REST status codes.
export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { isAdmin, permissions } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'groups:read')) {
		error(403, 'You do not have permission to view groups.');
	}

	const token = getAuthToken(cookies);
	const groups = token ? await listGroups(token) : [];
	// The full environment list, not the layout's session-scoped one -- an
	// Admin granting a *new* group access to an environment needs to see
	// every environment that exists, not just ones they've already been
	// granted (same "always fetch, permission-gate is separate" pattern
	// allPermissions already uses for the permission picker).
	const environments = token ? await listEnvironments(token) : [];

	return { groups, allPermissions: ALL_PERMISSIONS, environments };
};
