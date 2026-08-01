import { error } from '@sveltejs/kit';
import { hasPermission } from '$lib/permissions';
import type { PageServerLoad } from './$types';

// data.environments (from dashboard/+layout.server.ts) is already scoped to
// what this session can access -- Admin sees every environment, everyone
// else only what their groups grant -- so the checkboxes below never need
// to filter again with hasEnvironmentAccess themselves.
export const load: PageServerLoad = async ({ parent }) => {
	const { isAdmin, permissions, environments } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'flags:write')) {
		error(403, 'You do not have permission to create flags.');
	}

	return { environments };
};
