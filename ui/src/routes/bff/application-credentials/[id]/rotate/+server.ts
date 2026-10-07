import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '#lib/server/auth.ts';
import { hasPermission } from '#lib/permissions.ts';
import { rotateApplicationCredential } from '#lib/server/applicationCredentials.ts';
import { ErrorCode } from '#lib/errors.ts';
import type { RequestHandler } from './$types';

// Rotating a credential's secret is gated on applicationCredentials:update
// (not a dedicated permission) -- mirrors the backend's own choice
// (auth.PermApplicationCredentialsUpdate reused for the rotate route) to
// avoid a fifth permission string for what's fundamentally an update to the
// credential's secret material.
export const POST: RequestHandler = async ({ cookies, params }) => {
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

	const token = getAuthToken(cookies);
	const result = token
		? await rotateApplicationCredential(token, params.id)
		: { error: 'Not authenticated.', code: ErrorCode.AuthInvalidToken, status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(
		{ ...result.applicationCredential, clientSecret: result.clientSecret },
		{ status: 200 }
	);
};
