import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { deleteEnvironment, updateEnvironment } from '$lib/server/environments';
import { ErrorCode } from '$lib/errors';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'environments:update')) {
		return json(
			{
				error: 'You do not have permission to manage environments.',
				code: ErrorCode.AuthForbidden
			},
			{ status: 403 }
		);
	}

	const body = await request.json().catch(() => null);
	const name = typeof body?.name === 'string' ? body.name.trim() : '';

	if (!name) {
		return json(
			{ error: 'Name is required.', code: ErrorCode.BadRequestEnvironmentNameRequired },
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await updateEnvironment(token, params.id, { name })
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.environment, { status: 200 });
};

export const DELETE: RequestHandler = async ({ cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'environments:delete')) {
		return json(
			{
				error: 'You do not have permission to manage environments.',
				code: ErrorCode.AuthForbidden
			},
			{ status: 403 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await deleteEnvironment(token, params.id)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json({ status: 'deleted' });
};
