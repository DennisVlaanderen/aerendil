// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on GroupResult (lib/server/groups.ts) for why.
export type Failure = { error: string; status: number; code: string };

// send calls the backend and returns its JSON body, or a Failure carrying
// the backend's error, code and status (fallback used for a missing body).
export async function send(
	url: string,
	token: string,
	init: RequestInit,
	fallback: string
): Promise<{ body: unknown; failure?: undefined } | { body?: undefined; failure: Failure }> {
	const response = await fetch(url, {
		...init,
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` }
	});
	const payload = await response.json().catch(() => null);
	if (!response.ok) {
		return {
			failure: {
				error: typeof payload?.error === 'string' ? payload.error : fallback,
				code: typeof payload?.code === 'string' ? payload.code : 'internal',
				status: response.status
			}
		};
	}
	return payload
		? { body: payload }
		: { failure: { error: fallback, code: 'internal', status: 502 } };
}
