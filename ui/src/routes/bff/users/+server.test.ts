import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auth from '#lib/server/auth.ts';
import * as usersApi from '#lib/server/users.ts';
import { ErrorCode } from '#lib/errors.ts';
import { fakeCookies, jsonRequest, makeRequestEvent, makeSession } from '#lib/test/bff.ts';
import { POST } from './+server';

type CreateUserEvent = Parameters<typeof POST>[0];

afterEach(() => {
	vi.restoreAllMocks();
});

describe('POST /bff/users', () => {
	it('enforces users:create, preserves password whitespace, and never echoes password back', async () => {
		vi.spyOn(auth, 'getSession')
			.mockResolvedValueOnce(null)
			.mockResolvedValueOnce(makeSession({ permissions: ['users:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:create'] }))
			.mockResolvedValueOnce(makeSession({ permissions: ['users:create'] }));

		const createSpy = vi
			.spyOn(usersApi, 'createUser')
			.mockResolvedValueOnce({
				user: { id: 'u-9', username: 'bob', groupIds: ['g-1'], active: true }
			})
			.mockResolvedValueOnce({
				error: 'username taken',
				code: ErrorCode.ConflictUsernameTaken,
				status: 409
			});

		const forbidden = await POST(
			makeRequestEvent<CreateUserEvent>({
				request: jsonRequest({ username: 'bob', password: 'secret-password' })
			})
		);
		expect(forbidden.status).toBe(403);

		const badBody = await POST(
			makeRequestEvent<CreateUserEvent>({
				request: jsonRequest({ username: 'bob', password: '' })
			})
		);
		expect(badBody.status).toBe(400);
		expect(await badBody.json()).toEqual({
			error: 'Username and password are required.',
			code: ErrorCode.BadRequestUsernameRequired
		});

		const noToken = await POST(
			makeRequestEvent<CreateUserEvent>({
				request: jsonRequest({ username: 'bob', password: 'secret-password' }),
				cookies: fakeCookies()
			})
		);
		expect(noToken.status).toBe(401);

		const okRes = await POST(
			makeRequestEvent<CreateUserEvent>({
				request: jsonRequest({
					username: '  bob  ',
					password: '  spaced-secret  ',
					groupIds: ['g-1']
				}),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-usr' })
			})
		);
		const body = await okRes.json();
		expect(okRes.status).toBe(201);
		expect(body).toEqual({ id: 'u-9', username: 'bob', groupIds: ['g-1'], active: true });
		expect(body).not.toHaveProperty('password');
		expect(createSpy).toHaveBeenCalledWith('tok-usr', {
			username: 'bob',
			password: '  spaced-secret  ',
			groupIds: ['g-1']
		});

		const conflictRes = await POST(
			makeRequestEvent<CreateUserEvent>({
				request: jsonRequest({ username: 'bob', password: 'secret-password' }),
				cookies: fakeCookies({ 'aerendil.auth': 'tok-usr' })
			})
		);
		expect(conflictRes.status).toBe(409);
		expect(await conflictRes.json()).toEqual({
			error: 'username taken',
			code: ErrorCode.ConflictUsernameTaken
		});
	});
});
