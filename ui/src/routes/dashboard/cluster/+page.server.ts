import { error } from '@sveltejs/kit';
import { getAuthToken } from '#lib/server/auth.ts';
import { getClusterStatus } from '#lib/server/cluster.ts';
import type { PageServerLoad } from './$types';

// Admin-only, matching the backend's requireAdmin on /api/cluster/status.
export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { isAdmin } = await parent();
	if (!isAdmin) {
		error(403, 'Only administrators can view the cluster status.');
	}

	const token = getAuthToken(cookies);
	return { status: token ? await getClusterStatus(token) : null };
};
