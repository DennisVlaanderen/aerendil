import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSegment, deleteSegment, getSegment, listSegments, updateSegment } from './segments';
import { ErrorCode } from '#lib/errors.ts';

// Same convention as flags.test.ts: fetch is mocked.
function jsonResponse(status: number, body: unknown): Response {
	return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

beforeEach(() => {
	vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
	vi.unstubAllGlobals();
});

const segment = {
	environmentId: 'prod',
	key: 'beta',
	name: 'Beta testers',
	revision: 4,
	included: ['user-1'],
	excluded: [],
	rules: []
};

describe('listSegments', () => {
	it('returns the segments, or an empty list on a non-OK response', async () => {
		vi.mocked(fetch)
			.mockResolvedValueOnce(jsonResponse(200, { segments: [segment] }))
			.mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));

		expect(await listSegments('a-token', 'prod')).toEqual([segment]);
		expect(await listSegments('a-token', 'prod')).toEqual([]);
	});
});

describe('getSegment', () => {
	it('returns null when the segment is missing', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(404, { error: 'segment not found' }));

		expect(await getSegment('a-token', 'prod', 'beta')).toBeNull();
	});
});

describe('createSegment', () => {
	it('posts the segment', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(201, segment));

		expect(await createSegment('a-token', segment)).toEqual({ segment });
		const [url, init] = vi.mocked(fetch).mock.calls[0];
		expect(url).toEqual(expect.stringMatching(/\/api\/segments$/));
		expect(init).toEqual(expect.objectContaining({ method: 'POST' }));
	});
});

describe('updateSegment', () => {
	it('returns the backend error, code and status on a non-OK response', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(400, { error: 'no inSegment', code: ErrorCode.BadRequestSegmentUsesInSegment })
		);

		expect(await updateSegment('a-token', 'prod', 'beta', segment)).toEqual({
			error: 'no inSegment',
			code: ErrorCode.BadRequestSegmentUsesInSegment,
			status: 400
		});
		expect(vi.mocked(fetch).mock.calls[0][0]).toEqual(
			expect.stringContaining('/api/segments/beta?environmentId=prod')
		);
	});
});

describe('deleteSegment', () => {
	it('returns the in-use conflict', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(
			jsonResponse(409, { error: 'in use', code: ErrorCode.ConflictSegmentInUse })
		);

		expect(await deleteSegment('a-token', 'prod', 'beta')).toEqual({
			error: 'in use',
			code: ErrorCode.ConflictSegmentInUse,
			status: 409
		});
	});
});
