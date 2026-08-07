import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { createApplicationCredential } from '$lib/server/applicationCredentials';
import { ErrorCode } from '$lib/errors';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'applicationCredentials:create')) {
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
		? await createApplicationCredential(token, { name, environmentId, scopes })
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(
		{ ...result.applicationCredential, clientSecret: result.clientSecret },
		{ status: 201 }
	);
};
