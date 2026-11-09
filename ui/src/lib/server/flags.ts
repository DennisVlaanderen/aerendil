import { AERENDIL_API_ORIGIN as API_ORIGIN } from '$app/env/private';

export interface FlagSummary {
	environmentId: string;
	key: string;
	enabled: boolean;
	value: string;
	version: number;
}

// shortcut: today's flag pages still edit a flag as {enabled, value}. This
// bridge maps that onto the backend's docs/fqdp.md §6.1 definition, as a
// one-variation string flag. #147 replaces it with the variations, rules
// and segments editor and works with FlagDefinition directly.
interface FlagDefinition {
	environmentId: string;
	key: string;
	type: string;
	enabled: boolean;
	revision: number;
	variations: { id: string; value: unknown }[];
	[field: string]: unknown;
}

function toSummary(definition: FlagDefinition): FlagSummary {
	const value = definition.variations?.[0]?.value;
	return {
		environmentId: definition.environmentId,
		key: definition.key,
		enabled: definition.enabled,
		value: typeof value === 'string' ? value : JSON.stringify(value ?? ''),
		version: definition.revision
	};
}

const BRIDGE_VARIATION_ID = 'value';

function stringDefinition(key: string, enabled: boolean, value: string) {
	return {
		key,
		type: 'string',
		enabled,
		variations: [{ id: BRIDGE_VARIATION_ID, value }],
		offVariation: BRIDGE_VARIATION_ID,
		rules: [],
		fallthrough: { variation: BRIDGE_VARIATION_ID }
	};
}

// withSummaryEdit applies a summary edit to a stored definition, keeping its
// rules and other variations. Only a string flag's first value is editable.
function withSummaryEdit(
	definition: FlagDefinition,
	edit: { enabled: boolean; value: string }
): FlagDefinition {
	return {
		...definition,
		enabled: edit.enabled,
		variations: definition.variations.map((variation, i) =>
			i === 0 && definition.type === 'string' ? { ...variation, value: edit.value } : variation
		)
	};
}

// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on GroupResult (lib/server/groups.ts) for why.
export type FlagsResult =
	| { flags: FlagSummary[]; error?: undefined; status?: undefined; code?: undefined }
	| { flags?: undefined; error: string; status: number; code: string };

export async function listFlags(token: string, environmentId: string): Promise<FlagSummary[]> {
	const response = await fetch(
		`${API_ORIGIN}/api/flags?environmentId=${encodeURIComponent(environmentId)}`,
		{
			headers: { Authorization: `Bearer ${token}` }
		}
	).catch(() => null);
	if (!response?.ok) {
		// See the identical comment in lib/server/groups.ts's listGroups.
		console.error(`listFlags: backend returned ${response ? response.status : 'no response'}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.flags) ? payload.flags.map(toSummary) : [];
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
		body: JSON.stringify({
			...stringDefinition(input.key, input.enabled, input.value),
			environmentIds: input.environmentIds
		})
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error: typeof payload?.error === 'string' ? payload.error : "Couldn't create that flag.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.flags)
		? { flags: payload.flags.map(toSummary) }
		: { error: "Couldn't create that flag.", code: 'internal', status: 502 };
}

// `status` mirrors the backend's own HTTP status verbatim -- see FlagsResult.
export type FlagResult =
	| { flag: FlagSummary; error?: undefined; status?: undefined; code?: undefined }
	| { flag?: undefined; error: string; status: number; code: string };

export async function updateFlag(
	token: string,
	environmentId: string,
	key: string,
	input: { enabled: boolean; value: string }
): Promise<FlagResult> {
	const url = `${API_ORIGIN}/api/flags/${encodeURIComponent(key)}?environmentId=${encodeURIComponent(environmentId)}`;
	const failed = async (response: Response) => {
		const payload = await response.json().catch(() => null);
		return {
			error: typeof payload?.error === 'string' ? payload.error : "Couldn't update that flag.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	};

	// PUT replaces the whole definition, so read it first to keep its rules.
	const current = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
	if (!current.ok) {
		return failed(current);
	}
	const definition: FlagDefinition | null = await current.json().catch(() => null);
	if (!definition) {
		return { error: "Couldn't update that flag.", code: 'internal', status: 502 };
	}

	const response = await fetch(url, {
		method: 'PUT',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(withSummaryEdit(definition, input))
	});
	if (!response.ok) {
		return failed(response);
	}

	const flag = await response.json().catch(() => null);
	return flag
		? { flag: toSummary(flag) }
		: { error: "Couldn't update that flag.", code: 'internal', status: 502 };
}

export async function deleteFlag(
	token: string,
	environmentId: string,
	key: string
): Promise<{ error: string; status: number; code: string } | null> {
	const response = await fetch(
		`${API_ORIGIN}/api/flags/${encodeURIComponent(key)}?environmentId=${encodeURIComponent(environmentId)}`,
		{
			method: 'DELETE',
			headers: { Authorization: `Bearer ${token}` }
		}
	);
	if (response.ok) {
		return null;
	}

	const payload = await response.json().catch(() => null);
	return {
		error: typeof payload?.error === 'string' ? payload.error : "Couldn't delete that flag.",
		code: typeof payload?.code === 'string' ? payload.code : 'internal',
		status: response.status
	};
}
