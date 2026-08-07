import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	createApplicationCredential,
	deleteApplicationCredential,
	listApplicationCredentials,
	rotateApplicationCredential,
	updateApplicationCredential
} from './applicationCredentials';
import { ErrorCode } from '$lib/errors';

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

describe('listApplicationCredentials', () => {
	it('returns the credentials array on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, {
				applicationCredentials: [
					{
						id: 'c1',
						name: 'billing-service',
						environmentId: 'e1',
						scopes: ['flags:read'],
						active: true
					}
				]
			})
		);

		const result = await listApplicationCredentials('a-token', 'e1');

		expect(result).toEqual([
			{
				id: 'c1',
				name: 'billing-service',
				environmentId: 'e1',
				scopes: ['flags:read'],
				active: true
			}
		]);
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('environmentId=e1'),
			expect.anything()
		);
	});

	it('returns an empty array on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));

		const result = await listApplicationCredentials('a-token', 'e1');

		expect(result).toEqual([]);
	});
});

describe('createApplicationCredential', () => {
	it('returns the created credential and its plaintext secret on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(201, {
				id: 'c1',
				name: 'billing-service',
				environmentId: 'e1',
				scopes: ['flags:read'],
				active: true,
				clientSecret: 'super-secret'
			})
		);

		const result = await createApplicationCredential('a-token', {
			name: 'billing-service',
			environmentId: 'e1',
			scopes: ['flags:read']
		});

		expect(result).toEqual({
			applicationCredential: {
				id: 'c1',
				name: 'billing-service',
				environmentId: 'e1',
				scopes: ['flags:read'],
				active: true
			},
			clientSecret: 'super-secret'
		});
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(400, {
				error: 'name is required',
				code: ErrorCode.BadRequestCredentialNameRequired
			})
		);

		const result = await createApplicationCredential('a-token', {
			name: '',
			environmentId: 'e1',
			scopes: []
		});

		expect(result).toEqual({
			error: 'name is required',
			code: ErrorCode.BadRequestCredentialNameRequired,
			status: 400
		});
	});

	it('falls back to a generic error when the backend response is missing a clientSecret', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(201, {
				id: 'c1',
				name: 'billing-service',
				environmentId: 'e1',
				scopes: [],
				active: true
			})
		);

		const result = await createApplicationCredential('a-token', {
			name: 'billing-service',
			environmentId: 'e1',
			scopes: []
		});

		expect(result).toEqual({
			error: "Couldn't create that credential.",
			code: 'internal',
			status: 502
		});
	});
});

describe('updateApplicationCredential', () => {
	it('returns the updated credential on success, without a clientSecret', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, {
				id: 'c1',
				name: 'billing-service-renamed',
				environmentId: 'e1',
				scopes: ['flags:read', 'flags:write'],
				active: false
			})
		);

		const result = await updateApplicationCredential('a-token', 'c1', {
			name: 'billing-service-renamed',
			environmentId: 'e1',
			scopes: ['flags:read', 'flags:write'],
			active: false
		});

		expect(result).toEqual({
			applicationCredential: {
				id: 'c1',
				name: 'billing-service-renamed',
				environmentId: 'e1',
				scopes: ['flags:read', 'flags:write'],
				active: false
			}
		});
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/application-credentials/c1'),
			expect.objectContaining({ method: 'PUT' })
		);
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(404, {
				error: 'application credential not found',
				code: ErrorCode.NotFoundApplicationCredential
			})
		);

		const result = await updateApplicationCredential('a-token', 'does-not-exist', {
			name: 'x',
			environmentId: 'e1',
			scopes: [],
			active: true
		});

		expect(result).toEqual({
			error: 'application credential not found',
			code: ErrorCode.NotFoundApplicationCredential,
			status: 404
		});
	});
});

describe('rotateApplicationCredential', () => {
	it('returns the credential and its new plaintext secret on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(200, {
				id: 'c1',
				name: 'billing-service',
				environmentId: 'e1',
				scopes: ['flags:read'],
				active: true,
				clientSecret: 'new-secret'
			})
		);

		const result = await rotateApplicationCredential('a-token', 'c1');

		expect(result).toEqual({
			applicationCredential: {
				id: 'c1',
				name: 'billing-service',
				environmentId: 'e1',
				scopes: ['flags:read'],
				active: true
			},
			clientSecret: 'new-secret'
		});
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/application-credentials/c1/rotate'),
			expect.objectContaining({ method: 'POST' })
		);
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(404, { error: 'not found', code: ErrorCode.NotFoundApplicationCredential })
		);

		const result = await rotateApplicationCredential('a-token', 'does-not-exist');

		expect(result).toEqual({
			error: 'not found',
			code: ErrorCode.NotFoundApplicationCredential,
			status: 404
		});
	});
});

describe('deleteApplicationCredential', () => {
	it('returns null on success', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { status: 'deleted' }));

		const result = await deleteApplicationCredential('a-token', 'c1');

		expect(result).toBeNull();
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(403, { error: 'forbidden', code: ErrorCode.AuthForbidden })
		);

		const result = await deleteApplicationCredential('a-token', 'c1');

		expect(result).toEqual({ error: 'forbidden', code: ErrorCode.AuthForbidden, status: 403 });
	});
});
