import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { createGroup } from '$lib/server/groups';
import { ErrorCode } from '$lib/errors';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'groups:create')) {
		return json(
			{ error: 'You do not have permission to manage groups.', code: ErrorCode.AuthForbidden },
			{ status: 403 }
		);
	}

	const body = await request.json().catch(() => null);
	const name = typeof body?.name === 'string' ? body.name.trim() : '';
	const permissions = Array.isArray(body?.permissions) ? body.permissions.map(String) : [];
	const environmentIds = Array.isArray(body?.environmentIds) ? body.environmentIds.map(String) : [];

	if (!name) {
		return json(
			{ error: 'Name is required.', code: ErrorCode.BadRequestGroupNameRequired },
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await createGroup(token, { name, permissions, environmentIds })
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.group, { status: 201 });
};
