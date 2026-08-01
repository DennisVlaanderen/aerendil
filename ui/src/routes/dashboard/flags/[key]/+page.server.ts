import { error } from '@sveltejs/kit';
import { getAuthToken } from '$lib/server/auth';
import { listFlags } from '$lib/server/flags';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, params, parent }) => {
	const { selectedEnvironmentId } = await parent();
	const token = getAuthToken(cookies);
	const flags = token && selectedEnvironmentId ? await listFlags(token, selectedEnvironmentId) : [];
	const flag = flags.find((candidate) => candidate.key === params.key);

	if (!flag) {
		error(404, 'Flag not found');
	}

	return { flag };
};
