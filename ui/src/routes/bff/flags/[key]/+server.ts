import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { deleteFlag, updateFlag } from '$lib/server/flags';
import { ErrorCode } from '$lib/errors';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, cookies, params, url }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'flags:update')) {
		return json(
			{ error: 'You do not have permission to manage flags.', code: ErrorCode.AuthForbidden },
			{ status: 403 }
		);
	}

	const environmentId = url.searchParams.get('environmentId') ?? '';
	if (!environmentId) {
		return json(
			{
				error: 'environmentId is required.',
				code: ErrorCode.BadRequestFlagsEnvironmentIDRequired
			},
			{ status: 400 }
		);
	}

	const body = await request.json().catch(() => null);
	const enabled = Boolean(body?.enabled);
	const value = typeof body?.value === 'string' ? body.value : '';

	const token = getAuthToken(cookies);
	const result = token
		? await updateFlag(token, environmentId, params.key, { enabled, value })
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.flag, { status: 200 });
};

export const DELETE: RequestHandler = async ({ cookies, params, url }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'flags:delete')) {
		return json(
			{ error: 'You do not have permission to manage flags.', code: ErrorCode.AuthForbidden },
			{ status: 403 }
		);
	}

	const environmentId = url.searchParams.get('environmentId') ?? '';
	if (!environmentId) {
		return json(
			{
				error: 'environmentId is required.',
				code: ErrorCode.BadRequestFlagsEnvironmentIDRequired
			},
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await deleteFlag(token, environmentId, params.key)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json({ status: 'deleted' });
};
