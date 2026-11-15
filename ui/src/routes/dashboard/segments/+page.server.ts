import { error } from '@sveltejs/kit';
import { getAuthToken } from '#lib/server/auth.ts';
import { listSegments } from '#lib/server/segments.ts';
import { hasPermission } from '#lib/permissions.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { isAdmin, permissions, selectedEnvironmentId } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'segments:read')) {
		error(403, 'You do not have permission to view segments.');
	}
	const token = getAuthToken(cookies);
	const segments =
		token && selectedEnvironmentId ? await listSegments(token, selectedEnvironmentId) : [];
	return { segments };
};
