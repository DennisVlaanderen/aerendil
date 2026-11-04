import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, invalidJsonRequest, jsonRequest, makeRequestEvent } from '#lib/test/bff.ts';
import { POST } from './+server';

type LoginEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/login', () => {
	it('sets the auth cookie and omits the token from the response body on valid credentials', async () => {
		const loginSpy = vi.spyOn(auth, 'login').mockResolvedValueOnce({ token: 'secret-jwt-token' });
		const setCookieSpy = vi.spyOn(auth, 'setAuthCookie');
		const cookies = fakeCookies();

		const response = await POST(
			makeRequestEvent<LoginEvent>({
				request: jsonRequest({ username: '  admin  ', password: '  keep-spaces  ' }),
				cookies
			})
		);
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(body).toEqual({ success: true });
		expect(body).not.toHaveProperty('token');
		expect(loginSpy).toHaveBeenCalledWith('admin', '  keep-spaces  ');
		expect(setCookieSpy).toHaveBeenCalledWith(cookies, 'secret-jwt-token');
		expect(cookies.get('aerendil.auth')).toBe('secret-jwt-token');
	});

	it('returns 400 without calling the backend when username or password is missing or malformed', async () => {
		const loginSpy = vi.spyOn(auth, 'login');

		const missingRes = await POST(
			makeRequestEvent<LoginEvent>({
				request: jsonRequest({ username: '   ', password: 'pw' })
			})
		);
		expect(missingRes.status).toBe(400);
		expect(await missingRes.json()).toEqual({
			error: 'Invalid username or password.',
			code: ErrorCode.AuthInvalidCredentials
		});

		const malformedRes = await POST(
			makeRequestEvent<LoginEvent>({ request: invalidJsonRequest() })
		);
		expect(malformedRes.status).toBe(400);
		expect(loginSpy).not.toHaveBeenCalled();
	});

	it('returns 401 when the backend cannot be reached (login returns null)', async () => {
		vi.spyOn(auth, 'login').mockResolvedValueOnce(null);
		const cookies = fakeCookies();

		const response = await POST(
			makeRequestEvent<LoginEvent>({
				request: jsonRequest({ username: 'admin', password: 'pw' }),
				cookies
			})
		);

		expect(response.status).toBe(401);
		expect(await response.json()).toEqual({
			error: 'Invalid username or password.',
			code: ErrorCode.AuthInvalidCredentials
		});
		expect(cookies.set).not.toHaveBeenCalled();
	});

	it('propagates backend status and error body without setting a cookie', async () => {
		vi.spyOn(auth, 'login').mockResolvedValueOnce({
			error: 'account disabled',
			code: ErrorCode.AuthForbidden,
			status: 403
		});
		const cookies = fakeCookies();

		const response = await POST(
			makeRequestEvent<LoginEvent>({
				request: jsonRequest({ username: 'admin', password: 'pw' }),
				cookies
			})
		);

		expect(response.status).toBe(403);
		expect(await response.json()).toEqual({
			error: 'account disabled',
			code: ErrorCode.AuthForbidden
		});
		expect(cookies.set).not.toHaveBeenCalled();
	});
});
