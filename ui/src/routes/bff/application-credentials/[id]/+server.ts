import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import {
	deleteApplicationCredential,
	updateApplicationCredential
} from '$lib/server/applicationCredentials';
import { ErrorCode } from '$lib/errors';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'applicationCredentials:update')) {
		return json(
			{
				error: 'You do not have permission to manage application credentials.',
				code: ErrorCode.AuthForbidden
			},
			{ status: 403 }
		);
	}

	const body = await request.json().catch(() => null);
	const name = typeof body?.name === 'string' ? body.name.trim() : '';
	const environmentId = typeof body?.environmentId === 'string' ? body.environmentId : '';
	const scopes = Array.isArray(body?.scopes) ? body.scopes.map(String) : [];
	const active = body?.active === true;

	if (!name) {
		return json(
			{ error: 'Name is required.', code: ErrorCode.BadRequestCredentialNameRequired },
			{ status: 400 }
		);
	}
	if (!environmentId) {
		return json(
			{
				error: 'Environment is required.',
				code: ErrorCode.BadRequestCredentialEnvironmentRequired
			},
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await updateApplicationCredential(token, params.id, { name, environmentId, scopes, active })
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.applicationCredential, { status: 200 });
};

export const DELETE: RequestHandler = async ({ cookies, params }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'applicationCredentials:delete')) {
		return json(
			{
				error: 'You do not have permission to manage application credentials.',
				code: ErrorCode.AuthForbidden
			},
			{ status: 403 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await deleteApplicationCredential(token, params.id)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json({ status: 'deleted' });
};
