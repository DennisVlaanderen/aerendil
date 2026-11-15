import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as environmentsApi from '#lib/server/environments.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { POST } from './+server';

type CreateEnvEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/environments', () => {
	it('enforces environments:create and returns 401 when auth token is missing', async () => {
		const createSpy = vi.spyOn(environmentsApi, 'createEnvironment');
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:read'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['environments:create'] }));

		const unauth = await POST(
			makeRequestEvent<CreateEnvEvent>({ request: jsonRequest({ name: 'QA' }) })
		);
		expect(unauth.status).toBe(403);

		const forbidden = await POST(
			makeRequestEvent<CreateEnvEvent>({ request: jsonRequest({ name: 'QA' }) })
		);
		expect(forbidden.status).toBe(403);

		const missingToken = await POST(
			makeRequestEvent<CreateEnvEvent>({
				request: jsonRequest({ name: 'QA' }),
				cookies: fakeCookies()
			})
		);
		expect(missingToken.status).toBe(401);
		expect(await missingToken.json()).toEqual({
			error: 'Not authenticated.',
			code: ErrorCode.AuthInvalidToken
		});
		expect(createSpy).not.toHaveBeenCalled();
	});

	it('requires a non-empty name and forwards trimmed name and color to backend', async () => {
		vi.spyOn(auth, 'getSession').mockResolvedValue(
			makeSession({ permissions: ['environments:create'] })
		);
		const createSpy = vi
			.spyOn(environmentsApi, 'createEnvironment')
			.mockResolvedValueOnce({
				environment: { id: 'env-qa', name: 'QA', order: 2, color: '#22c55e' }
			})
			.mockResolvedValueOnce({
				error: 'invalid color',
				code: ErrorCode.BadRequestEnvironmentColorInvalid,
				status: 400
			});

		const emptyName = await POST(
			makeRequestEvent<CreateEnvEvent>({ request: jsonRequest({ name: '   ' }) })
		);
		expect(emptyName.status).toBe(400);
		expect(await emptyName.json()).toEqual({
			error: 'Name is required.',
			code: ErrorCode.BadRequestEnvironmentNameRequired
		});

		const okRes = await POST(
			makeRequestEvent<CreateEnvEvent>({
				request: jsonRequest({ name: '  QA  ', color: ' #22c55e ' }),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-env' })
			})
		);
		expect(okRes.status).toBe(201);
		expect(await okRes.json()).toEqual({
			id: 'env-qa',
			name: 'QA',
			order: 2,
			color: '#22c55e'
		});
		expect(createSpy).toHaveBeenCalledWith('tok-env', { name: 'QA', color: '#22c55e' });

		const errRes = await POST(
			makeRequestEvent<CreateEnvEvent>({
				request: jsonRequest({ name: 'QA', color: 'not-hex' }),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-env' })
			})
		);
		expect(errRes.status).toBe(400);
		expect(await errRes.json()).toEqual({
			error: 'invalid color',
			code: ErrorCode.BadRequestEnvironmentColorInvalid
		});
	});
});
