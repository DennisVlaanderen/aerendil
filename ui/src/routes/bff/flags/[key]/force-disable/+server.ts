import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { forceDisableFlag } from '#lib/server/flags.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ cookies, params, url }) => {
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

	const token = getAuthToken(cookies);
	const result = token
		? await forceDisableFlag(token, environmentId, params.key)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.flag, { status: 200 });
};
