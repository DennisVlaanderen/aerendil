import { env } from '$env/dynamic/private';

export interface FlagSummary {
	environmentId: string;
	key: string;
	enabled: boolean;
	value: string;
	version: number;
}

// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on GroupResult (lib/server/groups.ts) for why.
export type FlagsResult =
	| { flags: FlagSummary[]; error?: undefined; status?: undefined }
	| { flags?: undefined; error: string; status: number };

const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

export async function listFlags(token: string, environmentId: string): Promise<FlagSummary[]> {
	const response = await fetch(
		`${API_ORIGIN}/api/flags?environmentId=${encodeURIComponent(environmentId)}`,
		{
			headers: { Authorization: `Bearer ${token}` }
		}
	).catch(() => null);
	if (!response || !response.ok) {
		// See the identical comment in lib/server/groups.ts's listGroups.
		console.error(`listFlags: backend returned ${response ? response.status : 'no response'}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.flags) ? payload.flags : [];
}

// createFlags writes the same key/enabled/value into every listed
// environment as a single atomic backend request -- mirrors
// FlagRepository.SetMany's all-or-nothing contract; there is deliberately
// no single-environment create function, since "create in N environments
// at once, consistently" is the only supported shape.
export async function createFlags(
	token: string,
	input: { key: string; enabled: boolean; value: string; environmentIds: string[] }
): Promise<FlagsResult> {
	const response = await fetch(`${API_ORIGIN}/api/flags`, {
		method: 'POST',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error: typeof payload?.error === 'string' ? payload.error : "Couldn't create that flag.",
			status: response.status
		};
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.flags)
		? { flags: payload.flags }
		: { error: "Couldn't create that flag.", status: 502 };
}
