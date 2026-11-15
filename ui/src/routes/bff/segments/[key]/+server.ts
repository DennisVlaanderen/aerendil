import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { deleteSegment, updateSegment } from '#lib/server/segments.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

function forbidden() {
	return json(
		{ error: 'You do not have permission to manage segments.', code: ErrorCode.AuthForbidden },
		{ status: 403 }
	);
}

function environmentRequired() {
	return json(
		{
			error: 'environmentId is required.',
			code: ErrorCode.BadRequestSegmentEnvironmentIDRequired
		},
		{ status: 400 }
	);
}

export const PUT: RequestHandler = async ({ request, cookies, params, url }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'segments:update')) {
		return forbidden();
	}
	const environmentId = url.searchParams.get('environmentId') ?? '';
	if (!environmentId) {
		return environmentRequired();
	}

	const body = await request.json().catch(() => null);
	if (!body || typeof body !== 'object') {
		return json(
			{ error: 'Invalid request body.', code: ErrorCode.BadRequestBody },
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await updateSegment(token, environmentId, params.key, body)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.segment, { status: 200 });
};

export const DELETE: RequestHandler = async ({ cookies, params, url }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'segments:delete')) {
		return forbidden();
	}
	const environmentId = url.searchParams.get('environmentId') ?? '';
	if (!environmentId) {
		return environmentRequired();
	}

	const token = getAuthToken(cookies);
	const result = token
		? await deleteSegment(token, environmentId, params.key)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json({ status: 'deleted' });
};
