import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as flagsApi from '#lib/server/flags.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { DELETE, PUT } from './+server';

type PutFlagEvent = Parameters<typeof PUT>[0];
type DeleteFlagEvent = Parameters<typeof DELETE>[0];

const flag = {
	environmentId: 'env-prod',
	key: 'new-checkout',
	type: 'boolean' as const,
	enabled: true,
	revision: 3,
	variations: [
		{ id: 'true', value: true },
		{ id: 'false', value: false }
	],
	offVariation: 'false',
	rules: [],
	fallthrough: { variation: 'true' }
};

afterEach(() => {
	vi.restoreAllMocks();
});

describe('PUT /bff/flags/[key]', () => {
	it('enforces flags:update, requires environmentId query param, and forwards key and body', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:read'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:update'] }));

		const updateSpy = vi
			.spyOn(flagsApi, 'updateFlag')
			.mockResolvedValueOnce({
				flag
			})
			.mockResolvedValueOnce({
				error: 'flag not found',
				code: ErrorCode.NotFoundFlag,
				status: 404
			});

		const unauthenticated = await PUT(
			makeRequestEvent<PutFlagEvent>({
				params: { key: 'new-checkout' },
				url: 'http://localhost/bff/flags/new-checkout?environmentId=env-prod',
				request: jsonRequest({ enabled: true, revision: 3 }, 'PUT')
			})
		);
		expect(unauthenticated.status).toBe(403);

		const forbidden = await PUT(
			makeRequestEvent<PutFlagEvent>({
				params: { key: 'new-checkout' },
				url: 'http://localhost/bff/flags/new-checkout?environmentId=env-prod',
				request: jsonRequest({ enabled: true, revision: 3 }, 'PUT')
			})
		);
		expect(forbidden.status).toBe(403);

		const missingEnv = await PUT(
			makeRequestEvent<PutFlagEvent>({
				params: { key: 'new-checkout' },
				url: 'http://localhost/bff/flags/new-checkout',
				request: jsonRequest({ enabled: true, revision: 3 }, 'PUT')
			})
		);
		expect(missingEnv.status).toBe(400);
		expect(await missingEnv.json()).toEqual({
			error: 'environmentId is required.',
			code: ErrorCode.BadRequestFlagsEnvironmentIDRequired
		});

		const noToken = await PUT(
			makeRequestEvent<PutFlagEvent>({
				params: { key: 'new-checkout' },
				url: 'http://localhost/bff/flags/new-checkout?environmentId=env-prod',
				request: jsonRequest({ enabled: true, revision: 3 }, 'PUT'),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await PUT(
			makeRequestEvent<PutFlagEvent>({
				params: { key: 'new-checkout' },
				url: 'http://localhost/bff/flags/new-checkout?environmentId=env-prod',
				request: jsonRequest({ enabled: true, revision: 3 }, 'PUT'),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-f' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual(flag);
		expect(updateSpy).toHaveBeenCalledWith('tok-f', 'env-prod', 'new-checkout', {
			enabled: true,
			revision: 3
		});

		const errRes = await PUT(
			makeRequestEvent<PutFlagEvent>({
				params: { key: 'new-checkout' },
				url: 'http://localhost/bff/flags/new-checkout?environmentId=env-prod',
				request: jsonRequest({ enabled: false, revision: 3 }, 'PUT'),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-f' })
			})
		);
		expect(errRes.status).toBe(404);
		expect(await errRes.json()).toEqual({
			error: 'flag not found',
			code: ErrorCode.NotFoundFlag
		});
	});
});

describe('DELETE /bff/flags/[key]', () => {
	it('enforces flags:delete, validates environmentId query param, and forwards key and token', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:delete'] }));

		const deleteSpy = vi
			.spyOn(flagsApi, 'deleteFlag')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce({
				error: 'flag not found',
				code: ErrorCode.NotFoundFlag,
				status: 404
			});

		const forbidden = await DELETE(
			makeRequestEvent<DeleteFlagEvent>({
				params: { key: 'checkout' },
				url: 'http://localhost/bff/flags/checkout?environmentId=env-prod'
			})
		);
		expect(forbidden.status).toBe(403);

		const missingEnv = await DELETE(
			makeRequestEvent<DeleteFlagEvent>({
				params: { key: 'checkout' },
				url: 'http://localhost/bff/flags/checkout'
			})
		);
		expect(missingEnv.status).toBe(400);

		const noToken = await DELETE(
			makeRequestEvent<DeleteFlagEvent>({
				params: { key: 'checkout' },
				url: 'http://localhost/bff/flags/checkout?environmentId=env-prod',
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await DELETE(
			makeRequestEvent<DeleteFlagEvent>({
				params: { key: 'checkout' },
				url: 'http://localhost/bff/flags/checkout?environmentId=env-prod',
				cookies: fakeCookies({ 'aerendil.auth': 'tok-f' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({ status: 'deleted' });
		expect(deleteSpy).toHaveBeenCalledWith('tok-f', 'env-prod', 'checkout');

		const errRes = await DELETE(
			makeRequestEvent<DeleteFlagEvent>({
				params: { key: 'checkout' },
				url: 'http://localhost/bff/flags/checkout?environmentId=env-prod',
				cookies: fakeCookies({ 'aerendil.auth': 'tok-f' })
			})
		);
		expect(errRes.status).toBe(404);
	});
});
