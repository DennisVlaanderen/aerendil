import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	createEnvironment,
	deleteEnvironment,
	listEnvironments,
	updateEnvironment
} from './environments';

// Same convention as groups.test.ts: fetch is mocked, these are unit tests
// of the response-handling logic, not integration tests against a live
// backend.
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

describe('listEnvironments', () => {
	it('returns the environments array on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { environments: [{ id: 'prod', name: 'Production', order: 0 }] })
		);

		const result = await listEnvironments('a-token');

		expect(result).toEqual([{ id: 'prod', name: 'Production', order: 0 }]);
	});

	it('returns an empty array on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));

		const result = await listEnvironments('a-token');

		expect(result).toEqual([]);
	});
});

describe('createEnvironment', () => {
	it('returns the created environment on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, { id: 'e1', name: 'Staging', order: 1 })
		);

		const result = await createEnvironment('a-token', { name: 'Staging' });

		expect(result).toEqual({ environment: { id: 'e1', name: 'Staging', order: 1 } });
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(400, { error: 'name is required', code: 'BR03-0002' })
		);

		const result = await createEnvironment('a-token', { name: '' });

		expect(result).toEqual({ error: 'name is required', code: 'BR03-0002', status: 400 });
	});
});

describe('updateEnvironment', () => {
	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(404, { error: 'environment not found', code: 'NF03-0001' })
		);

		const result = await updateEnvironment('a-token', 'does-not-exist', { name: 'Renamed' });

		expect(result).toEqual({ error: 'environment not found', code: 'NF03-0001', status: 404 });
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/environments/does-not-exist'),
			expect.objectContaining({ method: 'PUT' })
		);
	});
});

describe('deleteEnvironment', () => {
	it('returns null on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { status: 'deleted' }));

		const result = await deleteEnvironment('a-token', 'e1');

		expect(result).toBeNull();
	});

	it('returns the backend error message and status when deleting the last environment is rejected', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(403, {
				error: 'cannot delete the last remaining environment',
				code: 'A03-0003'
			})
		);

		const result = await deleteEnvironment('a-token', 'prod');

		expect(result).toEqual({
			error: 'cannot delete the last remaining environment',
			code: 'A03-0003',
			status: 403
		});
	});
});
