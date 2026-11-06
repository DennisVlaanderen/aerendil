import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as groupsApi from '#lib/server/groups.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { POST } from './+server';

type CreateGroupEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/groups', () => {
	it('enforces groups:create, requires token, validates name, and forwards group payload', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:read'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:create'] }));

		const createSpy = vi
			.spyOn(groupsApi, 'createGroup')
			.mockResolvedValueOnce({
				group: {
					id: 'g-1',
					name: 'Platform',
					permissions: ['flags:read'],
					environmentIds: ['env-1'],
					system: false
				}
			})
			.mockResolvedValueOnce({
				error: 'unknown permission',
				code: ErrorCode.BadRequestUnknownPermission,
				status: 400
			});

		const unauthenticated = await POST(
			makeRequestEvent<CreateGroupEvent>({ request: jsonRequest({ name: 'Platform' }) })
		);
		expect(unauthenticated.status).toBe(403);

		const forbidden = await POST(
			makeRequestEvent<CreateGroupEvent>({ request: jsonRequest({ name: 'Platform' }) })
		);
		expect(forbidden.status).toBe(403);

		const badName = await POST(
			makeRequestEvent<CreateGroupEvent>({ request: jsonRequest({ name: '  ' }) })
		);
		expect(badName.status).toBe(400);
		expect(await badName.json()).toEqual({
			error: 'Name is required.',
			code: ErrorCode.BadRequestGroupNameRequired
		});

		const noToken = await POST(
			makeRequestEvent<CreateGroupEvent>({
				request: jsonRequest({ name: 'Platform' }),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await POST(
			makeRequestEvent<CreateGroupEvent>({
				request: jsonRequest({
					name: '  Platform  ',
					permissions: ['flags:read'],
					environmentIds: ['env-1']
				}),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-grp' })
			})
		);
		expect(okRes.status).toBe(201);
		expect(await okRes.json()).toEqual({
			id: 'g-1',
			name: 'Platform',
			permissions: ['flags:read'],
			environmentIds: ['env-1'],
			system: false
		});
		expect(createSpy).toHaveBeenCalledWith('tok-grp', {
			name: 'Platform',
			permissions: ['flags:read'],
			environmentIds: ['env-1']
		});

		const errRes = await POST(
			makeRequestEvent<CreateGroupEvent>({
				request: jsonRequest({ name: 'Platform', permissions: ['bogus'] }),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-grp' })
			})
		);
		expect(errRes.status).toBe(400);
		expect(await errRes.json()).toEqual({
			error: 'unknown permission',
			code: ErrorCode.BadRequestUnknownPermission
		});
	});
});
