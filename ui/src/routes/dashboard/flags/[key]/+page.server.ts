import { error } from '@sveltejs/kit';
import { getAuthToken } from '#lib/server/auth.ts';
import { getFlag } from '#lib/server/flags.ts';
import { listSegments } from '#lib/server/segments.ts';
import { hasPermission } from '#lib/permissions.ts';
import type { PageServerLoad } from './$types';

// A flag is edited per environment: the selected one (dashboard layout).
export const load: PageServerLoad = async ({ cookies, params, parent }) => {
	const { selectedEnvironmentId, isAdmin, permissions } = await parent();
	const token = getAuthToken(cookies);
	const flag =
		token && selectedEnvironmentId ? await getFlag(token, selectedEnvironmentId, params.key) : null;

	if (!flag || !token || !selectedEnvironmentId) {
		error(404, 'Flag not found');
	}

	// The inSegment picker; without segments:read it just lists none.
	const segments = hasPermission({ isAdmin, permissions }, 'segments:read')
		? await listSegments(token, selectedEnvironmentId)
		: [];

	return { flag, segmentKeys: segments.map((s) => s.key) };
};
