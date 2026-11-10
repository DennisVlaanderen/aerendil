import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as credsApi from '#lib/server/applicationCredentials.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { DELETE, PUT } from './+server';

type PutCredEvent = Parameters<typeof PUT>[0];
type DeleteCredEvent = Parameters<typeof DELETE>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('PUT /bff/application-credentials/[id]', () => {
	it('rejects unauthenticated or unauthorized callers with 403', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:read'] }));
		const updateSpy = vi.spyOn(credsApi, 'updateApplicationCredential');

		for (let i = 0; i < 2; i++) {
			const res = await PUT(
				makeRequestEvent<PutCredEvent>({
					params: { id: 'cred-1' },
					request: jsonRequest({ name: 'SDK', environmentId: 'env-1', active: true }, 'PUT')
				})
			);

			expect(res.status).toBe(403);
			expect(await res.json()).toEqual({
				error: 'You do not have permission to manage application credentials.',
				code: ErrorCode.AuthForbidden
			});
		}
		expect(updateSpy).not.toHaveBeenCalled();
	});

	it('validates name and environmentId and returns 401 when auth token is absent', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(
			makeSession({ permissions: ['applicationCredentials:update'] })
		);

		const badName = await PUT(
			makeRequestEvent<PutCredEvent>({
				params: { id: 'cred-1' },
				request: jsonRequest({ name: '', environmentId: 'env-1' }, 'PUT')
			})
		);
		expect(badName.status).toBe(400);
		expect(await badName.json()).toEqual({
			error: 'Name is required.',
			code: ErrorCode.BadRequestCredentialNameRequired
		});

		const badEnv = await PUT(
			makeRequestEvent<PutCredEvent>({
				params: { id: 'cred-1' },
				request: jsonRequest({ name: 'SDK', environmentId: '' }, 'PUT')
			})
		);
		expect(badEnv.status).toBe(400);

		const noToken = await PUT(
			makeRequestEvent<PutCredEvent>({
				params: { id: 'cred-1' },
				request: jsonRequest({ name: 'SDK', environmentId: 'env-1' }, 'PUT'),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);
		expect(await noToken.json()).toEqual({
			error: 'Not authenticated.',
			code: ErrorCode.AuthInvalidToken
		});
	});

	it('forwards token, id param, and body and maps success and backend errors', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(
			makeSession({ permissions: ['applicationCredentials:update'] })
		);
		const updateSpy = vi
			.spyOn(credsApi, 'updateApplicationCredential')
			.mockResolvedValueOnce({
				applicationCredential: {
					id: 'cred-42',
					name: 'Updated SDK',
					environmentId: 'env-1',
					scopes: ['flags:read'],
					active: true
				}
			})
			.mockResolvedValueOnce({
				error: 'not found',
				code: ErrorCode.NotFoundApplicationCredential,
				status: 404
			});

		const okRes = await PUT(
			makeRequestEvent<PutCredEvent>({
				params: { id: 'cred-42' },
				request: jsonRequest(
					{
						name: '  Updated SDK ',
						environmentId: 'env-1',
						scopes: ['flags:read'],
						active: true
					},
					'PUT'
				),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-xyz' })
			})
		);
		const body = await okRes.json();
		expect(okRes.status).toBe(200);
		expect(body).toEqual({
			id: 'cred-42',
			name: 'Updated SDK',
			environmentId: 'env-1',
			scopes: ['flags:read'],
			active: true
		});
		expect(body).not.toHaveProperty('clientSecret');
		expect(updateSpy).toHaveBeenCalledWith('tok-xyz', 'cred-42', {
			name: 'Updated SDK',
			environmentId: 'env-1',
			scopes: ['flags:read'],
			active: true
		});

		const errRes = await PUT(
			makeRequestEvent<PutCredEvent>({
				params: { id: 'cred-42' },
				request: jsonRequest({ name: 'Updated SDK', environmentId: 'env-1' }, 'PUT'),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-xyz' })
			})
		);
		expect(errRes.status).toBe(404);
		expect(await errRes.json()).toEqual({
			error: 'not found',
			code: ErrorCode.NotFoundApplicationCredential
		});
	});
});

describe('DELETE /bff/application-credentials/[id]', () => {
	it('enforces applicationCredentials:delete, requires auth token, and maps backend responses', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['applicationCredentials:delete'] }));

		const deleteSpy = vi
			.spyOn(credsApi, 'deleteApplicationCredential')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce({
				error: 'credential missing',
				code: ErrorCode.NotFoundApplicationCredential,
				status: 404
			});

		const forbidden = await DELETE(makeRequestEvent<DeleteCredEvent>({ params: { id: 'cred-9' } }));
		expect(forbidden.status).toBe(403);

		const noToken = await DELETE(
			makeRequestEvent<DeleteCredEvent>({
				params: { id: 'cred-9' },
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await DELETE(
			makeRequestEvent<DeleteCredEvent>({
				params: { id: 'cred-9' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-del' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({ status: 'deleted' });
		expect(deleteSpy).toHaveBeenCalledWith('tok-del', 'cred-9');

		const errRes = await DELETE(
			makeRequestEvent<DeleteCredEvent>({
				params: { id: 'cred-9' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-del' })
			})
		);
		expect(errRes.status).toBe(404);
		expect(await errRes.json()).toEqual({
			error: 'credential missing',
			code: ErrorCode.NotFoundApplicationCredential
		});
	});
});
