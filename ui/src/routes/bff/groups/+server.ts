import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { createGroup } from '$lib/server/groups';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'groups:create')) {
		return json({ error: 'You do not have permission to manage groups.' }, { status: 403 });
	}

	const body = await request.json().catch(() => null);
	const name = typeof body?.name === 'string' ? body.name.trim() : '';
	const permissions = Array.isArray(body?.permissions) ? body.permissions.map(String) : [];

	if (!name) {
		return json({ error: 'Name is required.' }, { status: 400 });
	}

	const token = getAuthToken(cookies);
	const result = token
		? await createGroup(token, { name, permissions })
		: { error: 'Not authenticated.', status: 401 };
	if (result.error) {
		return json({ error: result.error }, { status: result.status });
	}

	return json(result.group, { status: 201 });
};
