import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as usersApi from '#lib/server/users.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { DELETE, PUT } from './+server';

type PutUserEvent = Parameters<typeof PUT>[0];
type DeleteUserEvent = Parameters<typeof DELETE>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('PUT /bff/users/[id]', () => {
	it('enforces users:update, keeps omitted groupIds undefined, and forwards id and token', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['users:read'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:update'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:update'] }));

		const updateSpy = vi
			.spyOn(usersApi, 'updateUser')
			.mockResolvedValueOnce({
				user: { id: 'u-5', username: 'carol', groupIds: ['g-1'], active: true }
			})
			.mockResolvedValueOnce({
				error: 'last admin',
				code: ErrorCode.BusinessLastAdmin,
				status: 403
			});

		const unauthenticated = await PUT(
			makeRequestEvent<PutUserEvent>({
				params: { id: 'u-5' },
				request: jsonRequest({ username: 'carol', active: true }, 'PUT')
			})
		);
		expect(unauthenticated.status).toBe(403);

		const forbidden = await PUT(
			makeRequestEvent<PutUserEvent>({
				params: { id: 'u-5' },
				request: jsonRequest({ username: 'carol', active: true }, 'PUT')
			})
		);
		expect(forbidden.status).toBe(403);

		const badUsername = await PUT(
			makeRequestEvent<PutUserEvent>({
				params: { id: 'u-5' },
				request: jsonRequest({ username: '  ' }, 'PUT')
			})
		);
		expect(badUsername.status).toBe(400);

		const noToken = await PUT(
			makeRequestEvent<PutUserEvent>({
				params: { id: 'u-5' },
				request: jsonRequest({ username: 'carol', active: true }, 'PUT'),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await PUT(
			makeRequestEvent<PutUserEvent>({
				params: { id: 'u-5' },
				request: jsonRequest(
					{ username: '  carol ', password: ' new-pass ', active: true },
					'PUT'
				),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-usr' })
			})
		);
		const body = await okRes.json();
		expect(okRes.status).toBe(200);
		expect(body).toEqual({ id: 'u-5', username: 'carol', groupIds: ['g-1'], active: true });
		expect(body).not.toHaveProperty('password');
		expect(updateSpy).toHaveBeenCalledWith('tok-usr', 'u-5', {
			username: 'carol',
			password: ' new-pass ',
			groupIds: undefined,
			active: true
		});

		const errRes = await PUT(
			makeRequestEvent<PutUserEvent>({
				params: { id: 'u-5' },
				request: jsonRequest({ username: 'carol', groupIds: [], active: false }, 'PUT'),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-usr' })
			})
		);
		expect(errRes.status).toBe(403);
		expect(await errRes.json()).toEqual({
			error: 'last admin',
			code: ErrorCode.BusinessLastAdmin
		});
		expect(updateSpy).toHaveBeenLastCalledWith('tok-usr', 'u-5', {
			username: 'carol',
			password: undefined,
			groupIds: [],
			active: false
		});
	});
});

describe('DELETE /bff/users/[id]', () => {
	it('requires an administrator session even if users:delete permission is present', async () => {
		const deleteSpy = vi
			.spyOn(usersApi, 'deleteUser')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce({
				error: 'cannot delete last admin',
				code: ErrorCode.BusinessLastAdmin,
				status: 403
			});

		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(makeSession({ isAdmin: false, permissions: ['users:delete'] }))
			.mockResolvedValueOnce(makeSession({ isAdmin: true }))
			.mockResolvedValueOnce(makeSession({ isAdmin: true }))
			.mockResolvedValueOnce(makeSession({ isAdmin: true }));

		const nonAdmin = await DELETE(
			makeRequestEvent<DeleteUserEvent>({ params: { id: 'u-5' } })
		);
		expect(nonAdmin.status).toBe(403);
		expect(await nonAdmin.json()).toEqual({
			error: 'Only an administrator can delete users.',
			code: ErrorCode.BusinessAdminOnlyUserDelete
		});
		expect(deleteSpy).not.toHaveBeenCalled();

		const noToken = await DELETE(
			makeRequestEvent<DeleteUserEvent>({
				params: { id: 'u-5' },
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await DELETE(
			makeRequestEvent<DeleteUserEvent>({
				params: { id: 'u-5' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-admin' })
			})
		);
		expect(okRes.status).toBe(200);
		expect(await okRes.json()).toEqual({ status: 'deleted' });
		expect(deleteSpy).toHaveBeenCalledWith('tok-admin', 'u-5');

		const errRes = await DELETE(
			makeRequestEvent<DeleteUserEvent>({
				params: { id: 'u-1' },
				cookies: fakeCookies({ 'aerendil.auth': 'tok-admin' })
			})
		);
		expect(errRes.status).toBe(403);
		expect(await errRes.json()).toEqual({
			error: 'cannot delete last admin',
			code: ErrorCode.BusinessLastAdmin
		});
	});
});
