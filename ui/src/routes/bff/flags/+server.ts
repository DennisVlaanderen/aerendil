import { json } from '@sveltejs/kit';
import { getAuthToken, getSession } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { createFlags } from '$lib/server/flags';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const session = await getSession(cookies);
	if (!session || !hasPermission(session, 'flags:write')) {
		return json({ error: 'You do not have permission to manage flags.', code: 'A02-0001' }, { status: 403 });
	}

	const body = await request.json().catch(() => null);
	const key = typeof body?.key === 'string' ? body.key.trim() : '';
	const enabled = Boolean(body?.enabled);
	const value = typeof body?.value === 'string' ? body.value : '';
	const environmentIds = Array.isArray(body?.environmentIds) ? body.environmentIds.map(String) : [];

	if (!key) {
		return json({ error: 'Key is required.', code: 'BR02-0002' }, { status: 400 });
	}
	if (environmentIds.length === 0) {
		return json(
			{ error: 'At least one environment is required.', code: 'BR02-0003' },
			{ status: 400 }
		);
	}

	const token = getAuthToken(cookies);
	const result = token
		? await createFlags(token, { key, enabled, value, environmentIds })
		: { error: 'Not authenticated.', code: 'A01-0002', status: 401 };
	if (result.error) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	return json({ flags: result.flags }, { status: 200 });
};
