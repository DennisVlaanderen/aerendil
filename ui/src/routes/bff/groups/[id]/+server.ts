import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { deleteGroup, updateGroup } from '$lib/server/groups';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'groups:update')) {
		return json({ error: 'You do not have permission to manage groups.' }, { status: 403 });
	}

	// No pre-check for the Admin group here -- the backend is the single
	// source of truth (store.ErrProtectedSystemGroup, mapped to a 403), so
	// this route just forwards whatever it says instead of duplicating the
	// rule against a raw 'admin' string literal.
	const body = await request.json().catch(() => null);
	const name = typeof body?.name === 'string' ? body.name.trim() : '';
	const permissions = Array.isArray(body?.permissions) ? body.permissions.map(String) : [];
	const environmentIds = Array.isArray(body?.environmentIds) ? body.environmentIds.map(String) : [];

	if (!name) {
		return json({ error: 'Name is required.' }, { status: 400 });
	}

	const token = getAuthToken(cookies);
	const result = token
		? await updateGroup(token, params.id, { name, permissions, environmentIds })
		: { error: 'Not authenticated.', status: 401 };
	if (result.error) {
		return json({ error: result.error }, { status: result.status });
	}

	return json(result.group, { status: 200 });
};

export const DELETE: RequestHandler = async ({ cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'groups:delete')) {
		return json({ error: 'You do not have permission to manage groups.' }, { status: 403 });
	}

	// No pre-check for the Admin group here -- see the identical comment in
	// PUT above.
	const token = getAuthToken(cookies);
	const result = token
		? await deleteGroup(token, params.id)
		: { error: "Couldn't delete that group.", status: 401 };
	if (result) {
		return json({ error: result.error }, { status: result.status });
	}

	return json({ status: 'deleted' });
};
