import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import { ErrorCode } from '#lib/errors.ts';
import {
	fakeCookies,
	invalidJsonRequest,
	jsonRequest,
	makeRequestEvent,
	makeSession
} from '#lib/test/bff.ts';
import { POST } from './+server';

type SelectedEnvEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/selected-environment', () => {
	it('returns 401 when there is no active session', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValueOnce(null);
		const cookies = fakeCookies();

		const response = await POST(
			makeRequestEvent<SelectedEnvEvent>({
				request: jsonRequest({ environmentId: 'env-prod' }),
				cookies
			})
		);

		expect(response.status).toBe(401);
		expect(await response.json()).toEqual({
			error: 'Not authenticated.',
			code: ErrorCode.AuthInvalidToken
		});
		expect(cookies.set).not.toHaveBeenCalled();
	});

	it('rejects missing or inaccessible environment IDs for non-admin sessions with 403', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(
			makeSession({ isAdmin: false, environmentIds: ['env-dev'] })
		);
		const cookies = fakeCookies({ 'aerendil.auth': 'tok-123' });

		const forbiddenRes = await POST(
			makeRequestEvent<SelectedEnvEvent>({
				request: jsonRequest({ environmentId: 'env-prod' }),
				cookies
			})
		);
		expect(forbiddenRes.status).toBe(403);
		expect(await forbiddenRes.json()).toEqual({
			error: 'You do not have access to that environment.',
			code: ErrorCode.AuthForbidden
		});

		const missingFieldRes = await POST(
			makeRequestEvent<SelectedEnvEvent>({
				request: jsonRequest({}),
				cookies
			})
		);
		expect(missingFieldRes.status).toBe(403);

		const emptyRes = await POST(
			makeRequestEvent<SelectedEnvEvent>({
				request: invalidJsonRequest(),
				cookies
			})
		);
		expect(emptyRes.status).toBe(403);
		expect(cookies.set).not.toHaveBeenCalled();
	});

	it('stores the selected environment cookie for an accessible environment or admin user', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(makeSession({ isAdmin: false, environmentIds: ['env-stage'] }))
			.mockResolvedValueOnce(makeSession({ isAdmin: true, environmentIds: [] }));
		const setCookieSpy = vi.spyOn(auth, 'setSelectedEnvironmentCookie');
		const cookies = fakeCookies({ 'aerendil.auth': 'tok-123' });

		const memberRes = await POST(
			makeRequestEvent<SelectedEnvEvent>({
				request: jsonRequest({ environmentId: 'env-stage' }),
				cookies
			})
		);
		expect(memberRes.status).toBe(200);
		expect(await memberRes.json()).toEqual({ status: 'ok' });
		expect(setCookieSpy).toHaveBeenCalledWith(cookies, 'env-stage');

		const adminRes = await POST(
			makeRequestEvent<SelectedEnvEvent>({
				request: jsonRequest({ environmentId: 'env-any' }),
				cookies
			})
		);
		expect(adminRes.status).toBe(200);
		expect(cookies.get('aerendil.selected-environment')).toBe('env-any');
	});
});
