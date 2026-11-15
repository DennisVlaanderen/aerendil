import { AERENDIL_API_ORIGIN as API_ORIGIN } from '$app/env/private';
import type { Segment } from '#lib/flags.ts';

// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on GroupResult (lib/server/groups.ts) for why.
type Failure = { error: string; status: number; code: string };

export type SegmentResult =
	| { segment: Segment; error?: undefined; status?: undefined; code?: undefined }
	| ({ segment?: undefined } & Failure);

function segmentUrl(environmentId: string, key: string) {
	return `${API_ORIGIN}/api/segments/${encodeURIComponent(key)}?environmentId=${encodeURIComponent(environmentId)}`;
}

async function failure(response: Response, fallback: string): Promise<Failure> {
	const payload = await response.json().catch(() => null);
	return {
		error: typeof payload?.error === 'string' ? payload.error : fallback,
		code: typeof payload?.code === 'string' ? payload.code : 'internal',
		status: response.status
	};
}

async function write(
	url: string,
	token: string,
	method: 'POST' | 'PUT',
	segment: Partial<Segment>,
	fallback: string
): Promise<SegmentResult> {
	const response = await fetch(url, {
		method,
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(segment)
	});
	if (!response.ok) return failure(response, fallback);
	const applied = await response.json().catch(() => null);
	return applied ? { segment: applied } : { error: fallback, code: 'internal', status: 502 };
}

export async function listSegments(token: string, environmentId: string): Promise<Segment[]> {
	const response = await fetch(
		`${API_ORIGIN}/api/segments?environmentId=${encodeURIComponent(environmentId)}`,
		{ headers: { Authorization: `Bearer ${token}` } }
	).catch(() => null);
	if (!response?.ok) {
		// See the identical comment in lib/server/groups.ts's listGroups.
		console.error(`listSegments: backend returned ${response ? response.status : 'no response'}`);
		return [];
	}
	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.segments) ? payload.segments : [];
}

export async function getSegment(
	token: string,
	environmentId: string,
	key: string
): Promise<Segment | null> {
	const response = await fetch(segmentUrl(environmentId, key), {
		headers: { Authorization: `Bearer ${token}` }
	}).catch(() => null);
	return response?.ok ? response.json().catch(() => null) : null;
}

export function createSegment(token: string, segment: Partial<Segment>): Promise<SegmentResult> {
	return write(
		`${API_ORIGIN}/api/segments`,
		token,
		'POST',
		segment,
		"Couldn't create that segment."
	);
}

export function updateSegment(
	token: string,
	environmentId: string,
	key: string,
	segment: Partial<Segment>
): Promise<SegmentResult> {
	return write(
		segmentUrl(environmentId, key),
		token,
		'PUT',
		segment,
		"Couldn't update that segment."
	);
}

export async function deleteSegment(
	token: string,
	environmentId: string,
	key: string
): Promise<Failure | null> {
	const response = await fetch(segmentUrl(environmentId, key), {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});
	return response.ok ? null : failure(response, "Couldn't delete that segment.");
}
