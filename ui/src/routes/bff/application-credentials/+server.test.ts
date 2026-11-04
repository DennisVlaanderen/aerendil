import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as credsApi from '#lib/server/applicationCredentials.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { POST } from './+server';

type CreateCredEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/application-credentials', () => {
	it('rejects missing session or missing applicationCredentials:create permission with 403', async () => {
		const createSpy = vi.spyOn(credsApi, 'createApplicationCredential');
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:read'] }));

		for (let i = 0; i < 2; i++) {
			const res = await POST(
				makeRequestEvent<CreateCredEvent>({
					request: jsonRequest({ name: 'SDK', environmentId: 'env-1', scopes: ['flags:read'] })
				})
			);
			expect(res.status).toBe(403);
			expect(await res.json()).toEqual({
				error: 'You do not have permission to manage application credentials.',
				code: ErrorCode.AuthForbidden
			});
		}
		expect(createSpy).not.toHaveBeenCalled();
	});

	it('validates required name and environmentId fields before calling backend', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(
			makeSession({ permissions: ['applicationCredentials:create'] })
		);
		const createSpy = vi.spyOn(credsApi, 'createApplicationCredential');

		const noName = await POST(
			makeRequestEvent<CreateCredEvent>({
				request: jsonRequest({ name: '   ', environmentId: 'env-1' })
			})
		);
		expect(noName.status).toBe(400);
		expect(await noName.json()).toEqual({
			error: 'Name is required.',
			code: ErrorCode.BadRequestCredentialNameRequired
		});

		const noEnv = await POST(
			makeRequestEvent<CreateCredEvent>({
				request: jsonRequest({ name: 'SDK', environmentId: '' })
			})
		);
		expect(noEnv.status).toBe(400);
		expect(await noEnv.json()).toEqual({
			error: 'Environment is required.',
			code: ErrorCode.BadRequestCredentialEnvironmentRequired
		});
		expect(createSpy).not.toHaveBeenCalled();
	});

	it('returns 401 when the auth token cookie is missing', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValueOnce(
			makeSession({ permissions: ['applicationCredentials:create'] })
		);
		const createSpy = vi.spyOn(credsApi, 'createApplicationCredential');

		const res = await POST(
			makeRequestEvent<CreateCredEvent>({
				request: jsonRequest({ name: 'SDK', environmentId: 'env-1', scopes: ['flags:read'] }),
				cookies: fakeCookies()
			})
		);

		expect(res.status).toBe(401);
		expect(await res.json()).toEqual({
			error: 'Not authenticated.',
			code: ErrorCode.AuthInvalidToken
		});
		expect(createSpy).not.toHaveBeenCalled();
	});

	it('forwards token and payload and returns 201 with one-time clientSecret', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValueOnce(
			makeSession({ permissions: ['applicationCredentials:create'] })
		);
		const createSpy = vi.spyOn(credsApi, 'createApplicationCredential').mockResolvedValueOnce({
			applicationCredential: {
				id: 'cred-1',
				name: 'SDK Key',
				environmentId: 'env-1',
				scopes: ['flags:read'],
				active: true
			},
			clientSecret: 'sec_once_123'
		});

		const res = await POST(
			makeRequestEvent<CreateCredEvent>({
				request: jsonRequest({
					name: '  SDK Key  ',
					environmentId: 'env-1',
					scopes: ['flags:read']
				}),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-abc' })
			})
		);

		expect(res.status).toBe(201);
		expect(await res.json()).toEqual({
			id: 'cred-1',
			name: 'SDK Key',
			environmentId: 'env-1',
			scopes: ['flags:read'],
			active: true,
			clientSecret: 'sec_once_123'
		});
		expect(createSpy).toHaveBeenCalledWith('tok-abc', {
			name: 'SDK Key',
			environmentId: 'env-1',
			scopes: ['flags:read']
		});
	});

	it('maps backend errors to their HTTP status and error payload', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValueOnce(
			makeSession({ permissions: ['applicationCredentials:create'] })
		);
		vi.spyOn(credsApi, 'createApplicationCredential').mockResolvedValueOnce({
			error: 'unknown scope',
			code: ErrorCode.BadRequestUnknownScope,
			status: 400
		});

		const res = await POST(
			makeRequestEvent<CreateCredEvent>({
				request: jsonRequest({ name: 'SDK', environmentId: 'env-1', scopes: ['bad'] }),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-abc' })
			})
		);

		expect(res.status).toBe(400);
		expect(await res.json()).toEqual({
			error: 'unknown scope',
			code: ErrorCode.BadRequestUnknownScope
		});
	});
});
