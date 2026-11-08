import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as environmentsApi from '#lib/server/environments.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { DELETE, PUT } from './+server';

type PutEnvEvent = Parameters<typeof PUT>[0];
type DeleteEnvEvent = Parameters<typeof DELETE>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('PUT /bff/environments/[id]', () => {
	it('enforces environments:update, validates name, and omits absent color', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:read'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:update'] }));

		const updateSpy = vi.spyOn(environmentsApi, 'updateEnvironment').mockResolvedValueOnce({
			environment: { id: 'env-1', name: 'Staging', order: 1, color: '#3b82f6' }
		});

		const unauthenticated = await PUT(
			makeRequestEvent<PutEnvEvent>({
				params: { id: 'env-1' },
				request: jsonRequest({ name: 'Staging' }, 'PUT')
			})
		);
		expect(unauthenticated.status).toBe(403);

		const forbidden = await PUT(
			makeRequestEvent<PutEnvEvent>({
				params: { id: 'env-1' },
				request: jsonRequest({ name: 'Staging' }, 'PUT')
			})
		);
		expect(forbidden.status).toBe(403);

		const badName = await PUT(
			makeRequestEvent<PutEnvEvent>({
				params: { id: 'env-1' },
				request: jsonRequest({ name: '' }, 'PUT')
			})
		);
		expect(badName.status).toBe(400);

		const noToken = await PUT(
			makeRequestEvent<PutEnvEvent>({
				params: { id: 'env-1' },
				request: jsonRequest({ name: 'Staging' }, 'PUT'),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await PUT(
			makeRequestEvent<PutEnvEvent>({
				params: { id: 'env-1' },
				request: jsonRequest({ name: '  Staging  ' }, 'PUT'),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-env' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({
			id: 'env-1',
			name: 'Staging',
			order: 1,
			color: '#3b82f6'
		});
		expect(updateSpy).toHaveBeenCalledWith('tok-env', 'env-1', {
			name: 'Staging',
			color: undefined
		});
	});
});

describe('DELETE /bff/environments/[id]', () => {
	it('enforces environments:delete, requires auth token, and propagates backend restrictions', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:delete'] }));

		const deleteSpy = vi
			.spyOn(environmentsApi, 'deleteEnvironment')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce({
				error: 'cannot delete last environment',
				code: ErrorCode.BusinessLastEnvironment,
				status: 403
			});

		const forbidden = await DELETE(makeRequestEvent<DeleteEnvEvent>({ params: { id: 'env-1' } }));
		expect(forbidden.status).toBe(403);

		const noToken = await DELETE(
			makeRequestEvent<DeleteEnvEvent>({
				params: { id: 'env-1' },
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await DELETE(
			makeRequestEvent<DeleteEnvEvent>({
				params: { id: 'env-1' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-env' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({ status: 'deleted' });
		expect(deleteSpy).toHaveBeenCalledWith('tok-env', 'env-1');

		const errRes = await DELETE(
			makeRequestEvent<DeleteEnvEvent>({
				params: { id: 'env-1' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-env' })
			})
		);
		expect(errRes.status).toBe(403);
		expect(await errRes.json()).toEqual({
			error: 'cannot delete last environment',
			code: ErrorCode.BusinessLastEnvironment
		});
	});
});
