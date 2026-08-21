import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { deleteFlag, updateFlag } from './flags';
import { ErrorCode } from '$lib/errors';

// Same convention as environments.test.ts: fetch is mocked, these are unit
// tests of the response-handling logic, not integration tests against a
// live backend.
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

describe('updateFlag', () => {
	it('returns the updated flag on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, {
				environmentId: 'prod',
				key: 'checkout',
				enabled: false,
				value: 'off',
				version: 2
			})
		);

		const result = await updateFlag('a-token', 'prod', 'checkout', {
			enabled: false,
			value: 'off'
		});

		expect(result).toEqual({
			flag: { environmentId: 'prod', key: 'checkout', enabled: false, value: 'off', version: 2 }
		});
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/flags/checkout?environmentId=prod'),
			expect.objectContaining({ method: 'PUT' })
		);
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(404, { error: 'flag not found', code: ErrorCode.NotFoundFlag })
		);

		const result = await updateFlag('a-token', 'prod', 'does-not-exist', {
			enabled: true,
			value: ''
		});

		expect(result).toEqual({ error: 'flag not found', code: ErrorCode.NotFoundFlag, status: 404 });
	});
});

describe('deleteFlag', () => {
	it('returns null on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { status: 'deleted' }));

		const result = await deleteFlag('a-token', 'prod', 'checkout');

		expect(result).toBeNull();
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/flags/checkout?environmentId=prod'),
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(403, { error: 'forbidden', code: ErrorCode.AuthForbidden })
		);

		const result = await deleteFlag('a-token', 'prod', 'checkout');

		expect(result).toEqual({ error: 'forbidden', code: ErrorCode.AuthForbidden, status: 403 });
	});
});
