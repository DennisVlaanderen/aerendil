import { error } from '@sveltejs/kit';
import { getAuthToken } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { listEnvironments } from '$lib/server/environments';
import type { PageServerLoad } from './$types';

// Same layering as dashboard/groups/+page.server.ts: auth itself is already
// enforced by dashboard/+layout.server.ts; this load only adds the
// additional permission narrowing on top. Mutations (create/update/delete)
// live in bff/environments/+server.ts / [id]/+server.ts instead of form
// actions, so their responses carry real REST status codes.
export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { isAdmin, permissions } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'environments:read')) {
		error(403, 'You do not have permission to view this page.');
	}

	const token = getAuthToken(cookies);
	const environments = token ? await listEnvironments(token) : [];

	return { environments };
};
