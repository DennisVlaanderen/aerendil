import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as flagsApi from '#lib/server/flags.ts';
import { ErrorCode } from '#lib/errors.ts';
import {
	fakeCookies,
	makeRequestEvent,
	makeSession,
	type FakeRequestEventOptions
} from '#lib/test/bff.ts';
import { POST } from './+server';

type ForceDisableEvent = Parameters<typeof POST>[0];

const flag = {
	environmentId: 'env-prod',
	key: 'new-checkout',
	type: 'boolean' as const,
	enabled: false,
	revision: 4,
	variations: [
		{ id: 'true', value: true },
		{ id: 'false', value: false }
	],
	offVariation: 'false',
	rules: [],
	fallthrough: { variation: 'true' }
};

const url = 'http://localhost/bff/flags/new-checkout/force-disable?environmentId=env-prod';

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/flags/[key]/force-disable', () => {
	it('enforces flags:update, requires environmentId, and forwards key and token', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['flags:read'] }))
			.mockResolvedValue(makeSession({ permissions: ['flags:update'] }));

		const forceSpy = vi
			.spyOn(flagsApi, 'forceDisableFlag')
			.mockResolvedValueOnce({ flag })
			.mockResolvedValueOnce({
				error: 'flag not found',
				code: ErrorCode.NotFoundFlag,
				status: 404
			});
		const call = (overrides: FakeRequestEventOptions = {}) =>
			POST(
				makeRequestEvent<ForceDisableEvent>({
					params: { key: 'new-checkout' },
					url,
					request: new Request(url, { method: 'POST' }),
					...overrides
				})
			);

		expect((await call()).status).toBe(403);
		expect((await call()).status).toBe(403);

		const missingEnv = await call({ url: 'http://localhost/bff/flags/new-checkout/force-disable' });
		expect(missingEnv.status).toBe(400);
		expect(await missingEnv.json()).toEqual({
			error: 'environmentId is required.',
			code: ErrorCode.BadRequestFlagsEnvironmentIDRequired
		});

		expect((await call({ cookies: fakeCookies() })).status).toBe(401);

		const okRes = await call({ cookies: fakeCookies({ 'aerendil.auth': 'tok-f' }) });
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual(flag);
		expect(forceSpy).toHaveBeenCalledWith('tok-f', 'env-prod', 'new-checkout');

		const errRes = await call({ cookies: fakeCookies({ 'aerendil.auth': 'tok-f' }) });
		expect(errRes.status).toBe(404);
		expect(await errRes.json()).toEqual({ error: 'flag not found', code: ErrorCode.NotFoundFlag });
	});
});
