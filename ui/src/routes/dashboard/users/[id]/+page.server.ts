import { error } from '@sveltejs/kit';
import { getAuthToken } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { listUsers } from '$lib/server/users';
import { listGroups } from '$lib/server/groups';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, parent, params }) => {
	const { isAdmin, permissions } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'users:read')) {
		error(403, 'You do not have permission to view users.');
	}

	const token = getAuthToken(cookies);
	const [users, groups] = token
		? await Promise.all([listUsers(token), listGroups(token)])
		: [[], []];
	const user = users.find((candidate) => candidate.id === params.id);
	if (!user) {
		error(404, 'User not found');
	}

	return { user, groups };
};
