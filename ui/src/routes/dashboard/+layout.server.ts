import { redirect } from '@sveltejs/kit';
import { getAuthToken, getSelectedEnvironmentId, getSession } from '$lib/server/auth';
import { listEnvironments } from '$lib/server/environments';
import { listFlags } from '$lib/server/flags';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ cookies }) => {
	const session = await getSession(cookies);
	if (!session) {
		redirect(303, '/login');
	}

	const token = getAuthToken(cookies);
	const allEnvironments = token ? await listEnvironments(token) : [];

	// Environments this session can actually select -- Admin sees all,
	// mirroring the backend's unconditional bypass (auth/permissions.go's
	// resolvePermissionsForUser); everyone else only what their groups
	// grant. listEnvironments already comes back ordered by Order
	// ascending, so filtering preserves that order.
	const environments = session.isAdmin
		? allEnvironments
		: allEnvironments.filter((e) => session.environmentIds.includes(e.id));

	// Selected environment: the cookie's value if it's still one this
	// session can access, else the lowest-Order accessible environment,
	// else undefined if there are none -- flags/Sidebar render empty in
	// that last case rather than erroring.
	const cookieSelection = getSelectedEnvironmentId(cookies);
	const selectedEnvironmentId =
		environments.find((e) => e.id === cookieSelection)?.id ?? environments[0]?.id;

	const flags = token && selectedEnvironmentId ? await listFlags(token, selectedEnvironmentId) : [];

	return { ...session, flags, environments, selectedEnvironmentId };
};
