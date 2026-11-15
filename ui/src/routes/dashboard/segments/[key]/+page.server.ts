import { error } from '@sveltejs/kit';
import { getAuthToken } from '#lib/server/auth.ts';
import { getSegment } from '#lib/server/segments.ts';
import { hasPermission } from '#lib/permissions.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, params, parent }) => {
	const { isAdmin, permissions, selectedEnvironmentId } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'segments:read')) {
		error(403, 'You do not have permission to view segments.');
	}
	const token = getAuthToken(cookies);
	const segment =
		token && selectedEnvironmentId
			? await getSegment(token, selectedEnvironmentId, params.key)
			: null;
	if (!segment) {
		error(404, 'Segment not found');
	}
	return { segment };
};
