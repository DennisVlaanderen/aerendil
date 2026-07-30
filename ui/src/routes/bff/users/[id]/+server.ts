import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { deleteUser, updateUser } from '$lib/server/users';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'users:update')) {
		return json({ error: 'You do not have permission to manage users.' }, { status: 403 });
	}

	const body = await request.json().catch(() => null);
	const username = typeof body?.username === 'string' ? body.username.trim() : '';
	const password = typeof body?.password === 'string' ? body.password : '';
	// undefined (not []) when the field is absent -- distinguishes "the
	// caller didn't specify groups" from "the caller wants no groups", so
	// updateUser can omit the field and the backend leaves group membership
	// unchanged instead of clearing it.
	const groupIds = Array.isArray(body?.groupIds) ? body.groupIds.map(String) : undefined;
	const active = body?.active === true;

	if (!username) {
		return json({ error: 'Username is required.' }, { status: 400 });
	}

	const token = getAuthToken(cookies);
	const result = token
		? await updateUser(token, params.id, { username, password, groupIds, active })
		: { error: 'Not authenticated.', status: 401 };
	if (result.error) {
		return json({ error: result.error }, { status: result.status });
	}

	return json(result.user, { status: 200 });
};

export const DELETE: RequestHandler = async ({ cookies, params }) => {
	const session = await getSession(cookies);
	// Deleting a user is hardcoded to admins only, mirroring the identical
	// backend hardcode in usersDeleteHandler -- users:delete alone isn't
	// enough (unlike groups:delete, which has no such extra check).
	if (!session?.isAdmin) {
		return json({ error: 'Only an administrator can delete users.' }, { status: 403 });
	}

	const token = getAuthToken(cookies);
	const result = token
		? await deleteUser(token, params.id)
		: { error: "Couldn't delete that user.", status: 401 };
	if (result) {
		return json({ error: result.error }, { status: result.status });
	}

	return json({ status: 'deleted' });
};
