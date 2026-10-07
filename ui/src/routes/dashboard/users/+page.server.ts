import { error } from '@sveltejs/kit';
import { getAuthToken } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { listUsers } from '#lib/server/users.ts';
import { listGroups } from '#lib/server/groups.ts';
import type { PageServerLoad } from './$types';

// Auth itself is already enforced by dashboard/+layout.server.ts; this load
// only adds the additional permission narrowing on top, the same layering
// dashboard/flags/[key]/+page.server.ts already uses for its own 404 check.
// Mutations (create/update/delete) live in +server.ts / [id]/+server.ts
// instead of form actions, so their responses carry real REST status codes.
export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { isAdmin, permissions } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'users:read')) {
		error(403, 'You do not have permission to view users.');
	}

	const token = getAuthToken(cookies);
	const [users, groups] = token
		? await Promise.all([listUsers(token), listGroups(token)])
		: [[], []];

	return { users, groups };
};
