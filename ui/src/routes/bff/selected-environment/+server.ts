import { json } from '@sveltejs/kit';
import { getSession, setSelectedEnvironmentCookie } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session) {
		return json({ error: 'Not authenticated.', code: 'A01-0002' }, { status: 401 });
	}

	const body = await request.json().catch(() => null);
	const environmentId = typeof body?.environmentId === 'string' ? body.environmentId : '';

	// Re-validate against the caller's session-visible environments even
	// though the Header's selector already only offers those -- the cookie
	// is just a UI preference (see the comment on setSelectedEnvironmentCookie),
	// but a request forged with an ID the caller can't see should still be
	// rejected rather than silently scoping their view to it.
	if (!environmentId || (!session.isAdmin && !session.environmentIds.includes(environmentId))) {
		return json(
			{ error: 'You do not have access to that environment.', code: 'A02-0001' },
			{ status: 403 }
		);
	}

	setSelectedEnvironmentCookie(cookies, environmentId);
	return json({ status: 'ok' });
};
