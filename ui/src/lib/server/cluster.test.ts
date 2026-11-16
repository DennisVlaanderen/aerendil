import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getClusterStatus } from './cluster';

function jsonResponse(status: number, body: unknown): Response {
	return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

beforeEach(() => {
	vi.stubGlobal('fetch', vi.fn());
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

describe('getClusterStatus', () => {
	it('returns the status on success', async () => {
		const status = {
			leaderId: 'node-1',
			leaderAddress: '127.0.0.1:7000',
			nodes: [
				{ id: 'node-1', address: '127.0.0.1:7000', suffrage: 'Voter', leader: true, lag: null }
			],
			commitIndex: 5,
			lastLogIndex: 5,
			term: 2
		};
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, status));

		expect(await getClusterStatus('a-token')).toEqual(status);
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/api/cluster/status'),
			expect.objectContaining({ headers: { Authorization: 'Bearer a-token' } })
		);
	});

	it('returns null on a backend error', async () => {
		vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(403, { error: 'forbidden' }));
		expect(await getClusterStatus('a-token')).toBeNull();
	});

	it('returns null when the backend is unreachable', async () => {
		vi.mocked(fetch).mockRejectedValueOnce(new Error('offline'));
		expect(await getClusterStatus('a-token')).toBeNull();
	});
});
