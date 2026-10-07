import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { createEnvironment } from '#lib/server/environments.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'environments:create')) {
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
		? await createEnvironment(token, { name })
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.environment, { status: 201 });
};
