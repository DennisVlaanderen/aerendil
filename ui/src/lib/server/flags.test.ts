import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createFlags, deleteFlag, listFlags, updateFlag } from './flags';
import { ErrorCode } from '#lib/errors.ts';

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

// A docs/fqdp.md §6.1 definition as the backend returns it.
function definition(overrides: Record<string, unknown> = {}) {
	return {
		environmentId: 'prod',
		key: 'checkout',
		type: 'string',
		enabled: true,
		revision: 2,
		variations: [{ id: 'value', value: 'on' }],
		offVariation: 'value',
		rules: [{ id: 'beta', conditions: [], serve: { variation: 'value' } }],
		fallthrough: { variation: 'value' },
		...overrides
	};
}

describe('listFlags', () => {
	it('maps each definition to a summary', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { flags: [definition()] }));

		expect(await listFlags('a-token', 'prod')).toEqual([
			{ environmentId: 'prod', key: 'checkout', enabled: true, value: 'on', version: 2 }
		]);
	});
});

describe('createFlags', () => {
	it('sends a one-variation string definition', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { flags: [definition()] }));

		const result = await createFlags('a-token', {
			key: 'checkout',
			enabled: true,
			value: 'on',
			environmentIds: ['prod']
		});

		expect(result).toEqual({
			flags: [{ environmentId: 'prod', key: 'checkout', enabled: true, value: 'on', version: 2 }]
		});
		const body = JSON.parse(vi.mocked(fetch).mock.calls[0][1]?.body as string);
		expect(body).toEqual({
			key: 'checkout',
			type: 'string',
			enabled: true,
			variations: [{ id: 'value', value: 'on' }],
			offVariation: 'value',
			rules: [],
			fallthrough: { variation: 'value' },
			environmentIds: ['prod']
		});
	});
});

describe('updateFlag', () => {
	it('puts the stored definition back with the edit, keeping its rules', async () => {
		vi.mocked(fetch)
			.mockResolvedValueOnce(jsonResponse(200, definition()))
			.mockResolvedValueOnce(
				jsonResponse(
					200,
					definition({ enabled: false, variations: [{ id: 'value', value: 'off' }], revision: 3 })
				)
			);

		const result = await updateFlag('a-token', 'prod', 'checkout', {
			enabled: false,
			value: 'off'
		});

		expect(result).toEqual({
			flag: { environmentId: 'prod', key: 'checkout', enabled: false, value: 'off', version: 3 }
		});
		const [url, init] = vi.mocked(fetch).mock.calls[1];
		expect(url).toEqual(expect.stringContaining('/api/flags/checkout?environmentId=prod'));
		expect(init).toEqual(expect.objectContaining({ method: 'PUT' }));
		expect(JSON.parse(init?.body as string)).toEqual(
			definition({ enabled: false, variations: [{ id: 'value', value: 'off' }] })
		);
	});

	it('leaves the values of a non-string flag alone', async () => {
		const boolean = definition({
			type: 'boolean',
			variations: [
				{ id: 'true', value: true },
				{ id: 'false', value: false }
			]
		});
		vi.mocked(fetch)
			.mockResolvedValueOnce(jsonResponse(200, boolean))
			.mockResolvedValueOnce(jsonResponse(200, boolean));

		await updateFlag('a-token', 'prod', 'checkout', { enabled: false, value: 'ignored' });

		const body = JSON.parse(vi.mocked(fetch).mock.calls[1][1]?.body as string);
		expect(body.variations).toEqual(boolean.variations);
		expect(body.enabled).toBe(false);
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
		expect(fetch).toHaveBeenCalledTimes(1);
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
