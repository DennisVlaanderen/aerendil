import { error } from '@sveltejs/kit';
import { hasPermission } from '#lib/permissions.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
	const { isAdmin, permissions } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'segments:write')) {
		error(403, 'You do not have permission to create segments.');
	}
};
