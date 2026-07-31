import { redirect } from '@sveltejs/kit';
import { getAuthToken, getSelectedEnvironmentId, getSession } from '$lib/server/auth';
import { listFlags } from '$lib/server/flags';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ cookies }) => {
	const session = await getSession(cookies);
	if (!session) {
		redirect(303, '/login');
	}

	const token = getAuthToken(cookies);

	// Selected environment: the cookie's value if it's still one this
	// session can access, else the lowest-Order accessible environment,
	// else undefined if there are none -- flags/Sidebar render empty in
	// that last case rather than erroring. session.environments is already
	// exactly the set this session can access, resolved server-side by
	// /api/auth/me (Admin gets every environment, everyone else only what
	// their groups grant) -- no separate fetch/filter needed here.
	const cookieSelection = getSelectedEnvironmentId(cookies);
	const selectedEnvironmentId =
		session.environments.find((e) => e.id === cookieSelection)?.id ?? session.environments[0]?.id;

	const flags = token && selectedEnvironmentId ? await listFlags(token, selectedEnvironmentId) : [];

	return { ...session, flags, selectedEnvironmentId };
};
