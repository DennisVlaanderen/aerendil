import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { createSegment } from '#lib/server/segments.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'segments:write')) {
		return json(
			{ error: 'You do not have permission to manage segments.', code: ErrorCode.AuthForbidden },
			{ status: 403 }
		);
	}

	const body = await request.json().catch(() => null);
	if (!body || typeof body !== 'object') {
		return json(
			{ error: 'Invalid request body.', code: ErrorCode.BadRequestBody },
			{ status: 400 }
		);
	}
	if (typeof body.environmentId !== 'string' || !body.environmentId) {
		return json(
			{
				error: 'environmentId is required.',
				code: ErrorCode.BadRequestSegmentEnvironmentIDRequired
			},
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await createSegment(token, body)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.segment, { status: 201 });
};
