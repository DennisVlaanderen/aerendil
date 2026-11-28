import { AERENDIL_API_ORIGIN as API_ORIGIN } from '$app/env/private';
import type { EvaluationResult, FlagDefinition } from '#lib/flags.ts';
import { send, type Failure } from '#lib/server/backend.ts';

export type FlagsResult =
	| { flags: FlagDefinition[]; error?: undefined; status?: undefined; code?: undefined }
	| ({ flags?: undefined } & Failure);

export type FlagResult =
	| { flag: FlagDefinition; error?: undefined; status?: undefined; code?: undefined }
	| ({ flag?: undefined } & Failure);

export type EvaluationOutcome =
	| { result: EvaluationResult; error?: undefined; status?: undefined; code?: undefined }
	| ({ result?: undefined } & Failure);

function flagUrl(environmentId: string, key: string, suffix = '') {
	return `${API_ORIGIN}/api/flags/${encodeURIComponent(key)}${suffix}?environmentId=${encodeURIComponent(environmentId)}`;
}

export async function listFlags(token: string, environmentId: string): Promise<FlagDefinition[]> {
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
	return Array.isArray(payload?.flags) ? payload.flags : [];
}

export async function getFlag(
	token: string,
	environmentId: string,
	key: string
): Promise<FlagDefinition | null> {
	const { body } = await send(flagUrl(environmentId, key), token, {}, "Couldn't load that flag.");
	return (body as FlagDefinition) ?? null;
}

// createFlags writes one definition into every listed environment as a
// single atomic backend request -- mirrors FlagRepository.SetMany's
// all-or-nothing contract; there is deliberately no single-environment
// create function, since "create in N environments at once, consistently"
// is the only supported shape.
export async function createFlags(
	token: string,
	definition: Partial<FlagDefinition>,
	environmentIds: string[]
): Promise<FlagsResult> {
	const { body, failure } = await send(
		`${API_ORIGIN}/api/flags`,
		token,
		{ method: 'POST', body: JSON.stringify({ ...definition, environmentIds }) },
		"Couldn't create that flag."
	);
	if (failure) return failure;
	const flags = (body as { flags?: unknown }).flags;
	return Array.isArray(flags)
		? { flags }
		: { error: "Couldn't create that flag.", code: 'internal', status: 502 };
}

// updateFlag replaces the stored definition (PUT is a full replace; the
// backend keeps the salt and owns revision/schemaVersion).
export async function updateFlag(
	token: string,
	environmentId: string,
	key: string,
	definition: Partial<FlagDefinition>
): Promise<FlagResult> {
	const { body, failure } = await send(
		flagUrl(environmentId, key),
		token,
		{ method: 'PUT', body: JSON.stringify(definition) },
		"Couldn't update that flag."
	);
	return failure ?? { flag: body as FlagDefinition };
}

// evaluateFlag is the backend's dry run: it writes nothing.
export async function evaluateFlag(
	token: string,
	environmentId: string,
	key: string,
	context: unknown
): Promise<EvaluationOutcome> {
	const { body, failure } = await send(
		flagUrl(environmentId, key, '/evaluate'),
		token,
		{ method: 'POST', body: JSON.stringify(context) },
		"Couldn't evaluate that flag."
	);
	return failure ?? { result: body as EvaluationResult };
}

export async function deleteFlag(
	token: string,
	environmentId: string,
	key: string
): Promise<Failure | null> {
	const { failure } = await send(
		flagUrl(environmentId, key),
		token,
		{ method: 'DELETE' },
		"Couldn't delete that flag."
	);
	return failure ?? null;
}

// forceDisableFlag is the incident kill switch: it disables the flag without
// a revision check and is audited apart from a normal update.
export async function forceDisableFlag(
	token: string,
	environmentId: string,
	key: string
): Promise<FlagResult> {
	const { body, failure } = await send(
		flagUrl(environmentId, key, '/force-disable'),
		token,
		{ method: 'POST' },
		"Couldn't force-disable that flag."
	);
	return failure ?? { flag: body as FlagDefinition };
}
