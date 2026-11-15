import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { createFlags } from '#lib/server/flags.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'flags:write')) {
		return json(
			{ error: 'You do not have permission to manage flags.', code: ErrorCode.AuthForbidden },
			{ status: 403 }
		);
	}

	const body = await request.json().catch(() => null);
	const key = typeof body?.definition?.key === 'string' ? body.definition.key.trim() : '';
	const environmentIds: string[] = Array.isArray(body?.environmentIds)
		? body.environmentIds.map(String)
		: [];

	if (!key) {
		return json(
			{ error: 'Key is required.', code: ErrorCode.BadRequestFlagsKeyRequired },
			{ status: 400 }
		);
	}
	if (environmentIds.length === 0) {
		return json(
			{
				error: 'At least one environment is required.',
				code: ErrorCode.BadRequestFlagsEnvironmentIDsRequired
			},
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? // The backend validates the §6.1 definition itself (definitions.go).
			await createFlags(token, { ...body.definition, key }, environmentIds)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json({ flags: result.flags }, { status: 200 });
};
