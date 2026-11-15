import { AERENDIL_API_ORIGIN as API_ORIGIN } from '$app/env/private';
import type { Segment } from '#lib/flags.ts';
import { send, type Failure } from '#lib/server/backend.ts';

export type SegmentResult =
	| { segment: Segment; error?: undefined; status?: undefined; code?: undefined }
	| ({ segment?: undefined } & Failure);

function segmentUrl(environmentId: string, key: string) {
	return `${API_ORIGIN}/api/segments/${encodeURIComponent(key)}?environmentId=${encodeURIComponent(environmentId)}`;
}

async function write(
	url: string,
	token: string,
	method: 'POST' | 'PUT',
	segment: Partial<Segment>,
	fallback: string
): Promise<SegmentResult> {
	const { body, failure } = await send(
		url,
		token,
		{ method, body: JSON.stringify(segment) },
		fallback
	);
	return failure ?? { segment: body as Segment };
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
	// A backend that can't be reached shows as "not found", like a 404.
	const { body } = await send(segmentUrl(environmentId, key), token, {}, '').catch(() => ({
		body: undefined
	}));
	return (body as Segment) ?? null;
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
	const { failure } = await send(
		segmentUrl(environmentId, key),
		token,
		{ method: 'DELETE' },
		"Couldn't delete that segment."
	);
	return failure ?? null;
}
