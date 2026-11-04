import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as groupsApi from '#lib/server/groups.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { DELETE, PUT } from './+server';

type PutGroupEvent = Parameters<typeof PUT>[0];
type DeleteGroupEvent = Parameters<typeof DELETE>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('PUT /bff/groups/[id]', () => {
	it('enforces groups:update, validates name, requires token, and forwards id and payload', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:update'] }));

		const updateSpy = vi
			.spyOn(groupsApi, 'updateGroup')
			.mockResolvedValueOnce({
				group: {
					id: 'g-2',
					name: 'Developers',
					permissions: ['flags:read'],
					environmentIds: ['env-1'],
					system: false
				}
			})
			.mockResolvedValueOnce({
				error: 'protected system group',
				code: ErrorCode.BusinessProtectedGroup,
				status: 403
			});

		const forbidden = await PUT(
			makeRequestEvent<PutGroupEvent>({
				params: { id: 'g-2' },
				request: jsonRequest({ name: 'Developers' }, 'PUT')
			})
		);
		expect(forbidden.status).toBe(403);

		const badName = await PUT(
			makeRequestEvent<PutGroupEvent>({
				params: { id: 'g-2' },
				request: jsonRequest({ name: '' }, 'PUT')
			})
		);
		expect(badName.status).toBe(400);

		const noToken = await PUT(
			makeRequestEvent<PutGroupEvent>({
				params: { id: 'g-2' },
				request: jsonRequest({ name: 'Developers' }, 'PUT'),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await PUT(
			makeRequestEvent<PutGroupEvent>({
				params: { id: 'g-2' },
				request: jsonRequest(
					{
						name: '  Developers ',
						permissions: ['flags:read'],
						environmentIds: ['env-1']
					},
					'PUT'
				),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-grp' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(updateSpy).toHaveBeenCalledWith('tok-grp', 'g-2', {
			name: 'Developers',
			permissions: ['flags:read'],
			environmentIds: ['env-1']
		});

		const protectedRes = await PUT(
			makeRequestEvent<PutGroupEvent>({
				params: { id: 'admin' },
				request: jsonRequest({ name: 'Renamed' }, 'PUT'),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-grp' })
			})
		);
		expect(protectedRes.status).toBe(403);
		expect(await protectedRes.json()).toEqual({
			error: 'protected system group',
			code: ErrorCode.BusinessProtectedGroup
		});
	});
});

describe('DELETE /bff/groups/[id]', () => {
	it('enforces groups:delete, requires token, and propagates backend errors', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:delete'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['groups:delete'] }));

		const deleteSpy = vi
			.spyOn(groupsApi, 'deleteGroup')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce({
				error: 'protected group',
				code: ErrorCode.BusinessProtectedGroup,
				status: 403
			});

		const forbidden = await DELETE(makeRequestEvent<DeleteGroupEvent>({ params: { id: 'g-2' } }));
		expect(forbidden.status).toBe(403);

		const noToken = await DELETE(
			makeRequestEvent<DeleteGroupEvent>({
				params: { id: 'g-2' },
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await DELETE(
			makeRequestEvent<DeleteGroupEvent>({
				params: { id: 'g-2' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-grp' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({ status: 'deleted' });
		expect(deleteSpy).toHaveBeenCalledWith('tok-grp', 'g-2');

		const errRes = await DELETE(
			makeRequestEvent<DeleteGroupEvent>({
				params: { id: 'admin' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-grp' })
			})
		);
		expect(errRes.status).toBe(403);
		expect(await errRes.json()).toEqual({
			error: 'protected group',
			code: ErrorCode.BusinessProtectedGroup
		});
	});
});
