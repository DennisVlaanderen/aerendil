import { json } from '@sveltejs/kit';
import { login, setAuthCookie } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, cookies }) => {
	const body = await request.json().catch(() => null);
	const username = typeof body?.username === 'string' ? body.username.trim() : '';
	// Password is intentionally NOT trimmed -- user creation/update never
	// trims it either (backend/internal/api/users.go), so trimming only
	// here would silently lock out anyone whose password has meaningful
	// leading/trailing whitespace.
	const password = typeof body?.password === 'string' ? body.password : '';

	// "A01-0001" mirrors backend/internal/api.CodeAuthInvalidCredentials --
	// this is a local pre-check ahead of ever calling the backend, but the
	// user-facing failure is the same one login() itself would report for
	// bad credentials.
	if (!username || !password) {
		return json({ error: 'Invalid username or password.', code: 'A01-0001' }, { status: 400 });
	}

	const result = await login(username, password);
	if (!result) {
		return json({ error: 'Invalid username or password.', code: 'A01-0001' }, { status: 401 });
	}
	if ('error' in result) {
		return json({ error: result.error, code: result.code }, { status: result.status });
	}

	setAuthCookie(cookies, result.token);
	return json({ success: true });
};
