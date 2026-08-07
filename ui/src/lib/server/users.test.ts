import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createUser, deleteUser, listUsers, updateUser } from './users';
import { ErrorCode } from '$lib/errors';

// Same convention as auth.test.ts: fetch is mocked, these are unit tests of
// the response-handling logic, not integration tests against a live backend.
function jsonResponse(status: number, body: unknown): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		json: async () => body
	} as Response;
}

beforeEach(() => {
	vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('listUsers', () => {
	it('returns the users array on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { users: [{ id: 'u1', username: 'alice', groupIds: [], active: true }] })
		);

		const result = await listUsers('a-token');

		expect(result).toEqual([{ id: 'u1', username: 'alice', groupIds: [], active: true }]);
	});

	it('returns an empty array on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));

		const result = await listUsers('a-token');

		expect(result).toEqual([]);
	});

	it('returns an empty array when the payload is malformed', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { users: 'not-an-array' }));

		const result = await listUsers('a-token');

		expect(result).toEqual([]);
	});
});

describe('createUser', () => {
	it('returns the created user on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { id: 'u2', username: 'bob', groupIds: ['editors'], active: true })
		);

		const result = await createUser('a-token', {
			username: 'bob',
			password: 'secret',
			groupIds: ['editors']
		});

		expect(result).toEqual({
			user: { id: 'u2', username: 'bob', groupIds: ['editors'], active: true }
		});
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/users'),
			expect.objectContaining({ method: 'POST' })
		);
	});

	it('returns the backend error message on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(409, {
				error: 'username is already taken',
				code: ErrorCode.ConflictUsernameTaken
			})
		);

		const result = await createUser('a-token', {
			username: 'bob',
			password: 'secret',
			groupIds: []
		});

		expect(result).toEqual({
			error: 'username is already taken',
			code: ErrorCode.ConflictUsernameTaken,
			status: 409
		});
	});

	it('falls back to a generic message when the backend gives none', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(500, {}));

		const result = await createUser('a-token', {
			username: 'bob',
			password: 'secret',
			groupIds: []
		});

		expect(result).toEqual({
			error: "Couldn't create that user. The username may already be taken.",
			code: 'internal',
			status: 500
		});
	});
});

describe('updateUser', () => {
	it('returns the updated user on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { id: 'u1', username: 'alice2', groupIds: ['editors'], active: false })
		);

		const result = await updateUser('a-token', 'u1', {
			username: 'alice2',
			password: '',
			groupIds: ['editors'],
			active: false
		});

		expect(result).toEqual({
			user: { id: 'u1', username: 'alice2', groupIds: ['editors'], active: false }
		});
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/users/u1'),
			expect.objectContaining({ method: 'PUT' })
		);
	});

	it('returns the backend error message when the username is already taken', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(409, {
				error: 'username is already taken',
				code: ErrorCode.ConflictUsernameTaken
			})
		);

		const result = await updateUser('a-token', 'u1', {
			username: 'bob',
			password: '',
			groupIds: [],
			active: true
		});

		expect(result).toEqual({
			error: 'username is already taken',
			code: ErrorCode.ConflictUsernameTaken,
			status: 409
		});
	});
});

describe('deleteUser', () => {
	it('returns null on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { status: 'deleted' }));

		const result = await deleteUser('a-token', 'u1');

		expect(result).toBeNull();
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(403, {
				error: 'cannot remove the last remaining admin account',
				code: ErrorCode.BusinessLastAdmin
			})
		);

		const result = await deleteUser('a-token', 'u1');

		expect(result).toEqual({
			error: 'cannot remove the last remaining admin account',
			code: ErrorCode.BusinessLastAdmin,
			status: 403
		});
	});

	it('falls back to a generic message when the backend gives none', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(500, {}));

		const result = await deleteUser('a-token', 'u1');

		expect(result).toEqual({ error: "Couldn't delete that user.", code: 'internal', status: 500 });
	});
});
