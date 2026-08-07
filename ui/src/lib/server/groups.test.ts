import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createGroup, deleteGroup, listGroups, updateGroup } from './groups';

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

describe('listGroups', () => {
	it('returns the groups array on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { groups: [{ id: 'admin', name: 'Admin', permissions: [], system: true }] })
		);

		const result = await listGroups('a-token');

		expect(result).toEqual([{ id: 'admin', name: 'Admin', permissions: [], system: true }]);
	});

	it('returns an empty array on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));

		const result = await listGroups('a-token');

		expect(result).toEqual([]);
	});
});

describe('createGroup', () => {
	it('returns the created group on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { id: 'g1', name: 'Editors', permissions: ['flags:read'], system: false })
		);

		const result = await createGroup('a-token', {
			name: 'Editors',
			permissions: ['flags:read'],
			environmentIds: []
		});

		expect(result).toEqual({
			group: { id: 'g1', name: 'Editors', permissions: ['flags:read'], system: false }
		});
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(400, { error: 'unknown permission', code: 'BR05-0002' })
		);

		const result = await createGroup('a-token', {
			name: 'Editors',
			permissions: ['bogus'],
			environmentIds: []
		});

		expect(result).toEqual({ error: 'unknown permission', code: 'BR05-0002', status: 400 });
	});
});

describe('updateGroup', () => {
	it('returns the backend error message and status when the Admin group update is rejected', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(403, { error: 'the Admin group cannot be modified', code: 'A03-0002' })
		);

		const result = await updateGroup('a-token', 'admin', {
			name: 'Renamed',
			permissions: [],
			environmentIds: []
		});

		expect(result).toEqual({
			error: 'the Admin group cannot be modified',
			code: 'A03-0002',
			status: 403
		});
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/groups/admin'),
			expect.objectContaining({ method: 'PUT' })
		);
	});
});

describe('deleteGroup', () => {
	it('returns null on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { status: 'deleted' }));

		const result = await deleteGroup('a-token', 'g1');

		expect(result).toBeNull();
	});

	it('returns the backend error message and status when the Admin group delete is rejected', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(403, { error: 'the Admin group cannot be modified', code: 'A03-0002' })
		);

		const result = await deleteGroup('a-token', 'admin');

		expect(result).toEqual({
			error: 'the Admin group cannot be modified',
			code: 'A03-0002',
			status: 403
		});
	});
});
