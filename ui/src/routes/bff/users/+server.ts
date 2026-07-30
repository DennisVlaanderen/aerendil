import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { createUser } from '$lib/server/users';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'users:create')) {
		return json({ error: 'You do not have permission to create users.' }, { status: 403 });
	}

	const body = await request.json().catch(() => null);
	const username = typeof body?.username === 'string' ? body.username.trim() : '';
	const password = typeof body?.password === 'string' ? body.password : '';
	const groupIds = Array.isArray(body?.groupIds) ? body.groupIds.map(String) : [];

	if (!username || !password) {
		return json({ error: 'Username and password are required.' }, { status: 400 });
	}

	const token = getAuthToken(cookies);
	const result = token
		? await createUser(token, { username, password, groupIds })
		: { error: 'Not authenticated.', status: 401 };
	if (result.error) {
		return json({ error: result.error }, { status: result.status });
	}

	return json(result.user, { status: 201 });
};
