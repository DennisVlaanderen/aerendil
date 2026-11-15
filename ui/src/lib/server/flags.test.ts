import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createFlags, deleteFlag, evaluateFlag, getFlag, listFlags, updateFlag } from './flags';
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
		type: 'string' as const,
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
	it('returns the definitions as the backend sends them', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { flags: [definition()] }));

		expect(await listFlags('a-token', 'prod')).toEqual([definition()]);
	});

	it('returns an empty list on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));

		expect(await listFlags('a-token', 'prod')).toEqual([]);
	});
});

describe('getFlag', () => {
	it('returns the definition, or null when missing', async () => {
		vi.mocked(fetch)
			.mockResolvedValueOnce(jsonResponse(200, definition()))
			.mockResolvedValueOnce(jsonResponse(404, { error: 'flag not found' }));

		expect(await getFlag('a-token', 'prod', 'checkout')).toEqual(definition());
		expect(await getFlag('a-token', 'prod', 'missing')).toBeNull();
		expect(vi.mocked(fetch).mock.calls[0][0]).toEqual(
			expect.stringContaining('/api/flags/checkout?environmentId=prod')
		);
	});
});

describe('createFlags', () => {
	it('sends the definition with the environment ids', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { flags: [definition()] }));
		const input = { ...definition(), environmentId: undefined, revision: undefined };

		const result = await createFlags('a-token', input, ['prod', 'dev']);

		expect(result).toEqual({ flags: [definition()] });
		const body = JSON.parse(vi.mocked(fetch).mock.calls[0][1]?.body as string);
		// JSON drops the undefined fields the backend owns.
		expect(body).toEqual({ ...JSON.parse(JSON.stringify(input)), environmentIds: ['prod', 'dev'] });
	});

	it('returns the backend error, code and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(409, { error: 'flag already exists', code: ErrorCode.ConflictFlagExists })
		);

		expect(await createFlags('a-token', { key: 'checkout' }, ['prod'])).toEqual({
			error: 'flag already exists',
			code: ErrorCode.ConflictFlagExists,
			status: 409
		});
	});
});

describe('updateFlag', () => {
	it('puts the definition as given in one request', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, definition({ revision: 3 })));
		const edited = definition({ enabled: false, rules: [] });

		const result = await updateFlag('a-token', 'prod', 'checkout', edited);

		expect(result).toEqual({ flag: definition({ revision: 3 }) });
		expect(fetch).toHaveBeenCalledTimes(1);
		const [url, init] = vi.mocked(fetch).mock.calls[0];
		expect(url).toEqual(expect.stringContaining('/api/flags/checkout?environmentId=prod'));
		expect(init).toEqual(expect.objectContaining({ method: 'PUT' }));
		expect(JSON.parse(init?.body as string)).toEqual(edited);
	});

	it('returns the backend error message and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(400, {
				error: 'weights must sum to 100000',
				code: ErrorCode.BadRequestFlagRolloutWeightsInvalid
			})
		);

		const result = await updateFlag('a-token', 'prod', 'checkout', definition());

		expect(result).toEqual({
			error: 'weights must sum to 100000',
			code: ErrorCode.BadRequestFlagRolloutWeightsInvalid,
			status: 400
		});
	});
});

describe('evaluateFlag', () => {
	it('posts the context to the dry-run endpoint', async () => {
		const evaluation = { value: 'on', variationId: 'value', reason: 'FALLTHROUGH' };
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, evaluation));
		const context = { targetingKey: 'user-1', attributes: { plan: 'pro' } };

		expect(await evaluateFlag('a-token', 'prod', 'checkout', context)).toEqual({
			result: evaluation
		});
		const [url, init] = vi.mocked(fetch).mock.calls[0];
		expect(url).toEqual(expect.stringContaining('/api/flags/checkout/evaluate?environmentId=prod'));
		expect(JSON.parse(init?.body as string)).toEqual(context);
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
