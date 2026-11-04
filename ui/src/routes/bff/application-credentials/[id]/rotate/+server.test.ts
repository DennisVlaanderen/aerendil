import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as credsApi from '#lib/server/applicationCredentials.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { POST } from './+server';

type RotateCredEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/application-credentials/[id]/rotate', () => {
	it('gates rotation on applicationCredentials:update and rejects missing tokens with 401', async () => {
		const rotateSpy = vi.spyOn(credsApi, 'rotateApplicationCredential');
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:update'] }));

		const unauth = await POST(makeRequestEvent<RotateCredEvent>({ params: { id: 'cred-1' } }));
		expect(unauth.status).toBe(403);

		const wrongPerm = await POST(makeRequestEvent<RotateCredEvent>({ params: { id: 'cred-1' } }));
		expect(wrongPerm.status).toBe(403);

		const missingTok = await POST(
			makeRequestEvent<RotateCredEvent>({
				params: { id: 'cred-1' },
				cookies: fakeCookies()
			})
		);
		expect(missingTok.status).toBe(401);
		expect(await missingTok.json()).toEqual({
			error: 'Not authenticated.',
			code: ErrorCode.AuthInvalidToken
		});
		expect(rotateSpy).not.toHaveBeenCalled();
	});

	it('forwards token and credential id and propagates rotated secret and backend errors', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(
			makeSession({ permissions: ['applicationCredentials:update'] })
		);
		const rotateSpy = vi
			.spyOn(credsApi, 'rotateApplicationCredential')
			.mockResolvedValueOnce({
				applicationCredential: {
					id: 'cred-7',
					name: 'Runner',
					environmentId: 'env-1',
					scopes: ['flags:read'],
					active: true
				},
				clientSecret: 'rotated-secret'
			})
			.mockResolvedValueOnce({
				error: 'credential not found',
				code: ErrorCode.NotFoundApplicationCredential,
				status: 404
			});

		const okRes = await POST(
			makeRequestEvent<RotateCredEvent>({
				params: { id: 'cred-7' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-rot' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({
			id: 'cred-7',
			name: 'Runner',
			environmentId: 'env-1',
			scopes: ['flags:read'],
			active: true,
			clientSecret: 'rotated-secret'
		});
		expect(rotateSpy).toHaveBeenCalledWith('tok-rot', 'cred-7');

		const errRes = await POST(
			makeRequestEvent<RotateCredEvent>({
				params: { id: 'cred-7' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-rot' })
			})
		);
		expect(errRes.status).toBe(404);
		expect(await errRes.json()).toEqual({
			error: 'credential not found',
			code: ErrorCode.NotFoundApplicationCredential
		});
	});
});
