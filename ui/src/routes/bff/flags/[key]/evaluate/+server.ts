import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { evaluateFlag } from '#lib/server/flags.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

// A dry run against the stored flag: read permission, like the backend route.
export const POST: RequestHandler = async ({ request, cookies, params, url }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'flags:read')) {
		return json(
			{ error: 'You do not have permission to read flags.', code: ErrorCode.AuthForbidden },
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

	const context = await request.json().catch(() => null);
	const token = getAuthToken(cookies);
	const result = token
		? await evaluateFlag(token, environmentId, params.key, context)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.result, { status: 200 });
};
