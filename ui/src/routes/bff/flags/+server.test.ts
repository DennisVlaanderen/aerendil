import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as flagsApi from '#lib/server/flags.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { POST } from './+server';

type CreateFlagEvent = Parameters<typeof POST>[0];

const flag = {
	environmentId: 'env-1',
	key: 'checkout',
	type: 'boolean' as const,
	enabled: true,
	revision: 1,
	variations: [
		{ id: 'true', value: true },
		{ id: 'false', value: false }
	],
	offVariation: 'false',
	rules: [],
	fallthrough: { variation: 'false' }
};

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/flags', () => {
	it('enforces flags:write and returns 401 when auth token is absent', async () => {
		const createSpy = vi.spyOn(flagsApi, 'createFlags');
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:read'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:write'] }));

		const unauth = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({ definition: { key: 'checkout' }, environmentIds: ['env-1'] })
			})
		);
		expect(unauth.status).toBe(403);

		const forbidden = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({ definition: { key: 'checkout' }, environmentIds: ['env-1'] })
			})
		);
		expect(forbidden.status).toBe(403);

		const missingToken = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({ definition: { key: 'checkout' }, environmentIds: ['env-1'] }),
				cookies: fakeCookies()
			})
		);
		expect(missingToken.status).toBe(401);
		expect(createSpy).not.toHaveBeenCalled();
	});

	it('validates key and environmentIds and forwards payload and backend errors', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(makeSession({ permissions: ['flags:write'] }));
		const createSpy = vi
			.spyOn(flagsApi, 'createFlags')
			.mockResolvedValueOnce({
				flags: [flag]
			})
			.mockResolvedValueOnce({
				error: 'flag already exists',
				code: ErrorCode.ConflictFlagExists,
				status: 409
			});

		const missingKey = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({ definition: { key: '   ' }, environmentIds: ['env-1'] })
			})
		);
		expect(missingKey.status).toBe(400);
		expect(await missingKey.json()).toEqual({
			error: 'Key is required.',
			code: ErrorCode.BadRequestFlagsKeyRequired
		});

		const missingEnvs = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({ definition: { key: 'checkout' }, environmentIds: [] })
			})
		);
		expect(missingEnvs.status).toBe(400);
		expect(await missingEnvs.json()).toEqual({
			error: 'At least one environment is required.',
			code: ErrorCode.BadRequestFlagsEnvironmentIDsRequired
		});

		const okRes = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({
					definition: { key: '  checkout  ', type: 'boolean', enabled: true },
					environmentIds: ['env-1']
				}),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-flags' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({ flags: [flag] });
		expect(createSpy).toHaveBeenCalledWith(
			'tok-flags',
			{ key: 'checkout', type: 'boolean', enabled: true },
			['env-1']
		);

		const conflictRes = await POST(
			makeRequestEvent<CreateFlagEvent>({
				request: jsonRequest({ definition: { key: 'checkout' }, environmentIds: ['env-1'] }),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-flags' })
			})
		);
		expect(conflictRes.status).toBe(409);
		expect(await conflictRes.json()).toEqual({
			error: 'flag already exists',
			code: ErrorCode.ConflictFlagExists
		});
	});
});
