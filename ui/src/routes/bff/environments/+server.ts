import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { createEnvironment } from '$lib/server/environments';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'environments:create')) {
		return json(
			{ error: 'You do not have permission to manage environments.', code: 'A02-0001' },
			{ status: 403 }
		);
	}

	const body = await request.json().catch(() => null);
	const name = typeof body?.name === 'string' ? body.name.trim() : '';

	if (!name) {
		return json({ error: 'Name is required.', code: 'BR03-0002' }, { status: 400 });
	}

	const token = getAuthToken(cookies);
	const result = token
		? await createEnvironment(token, { name })
		: { error: 'Not authenticated.', code: 'A01-0002', status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json(result.environment, { status: 201 });
};
