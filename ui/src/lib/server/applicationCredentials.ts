import { env } from '$env/dynamic/private';

export interface ApplicationCredentialSummary {
	id: string;
	name: string;
	environmentId: string;
	scopes: string[];
	active: boolean;
}

// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on UserResult (lib/server/users.ts) for why.
export type ApplicationCredentialResult =
	| {
			applicationCredential: ApplicationCredentialSummary;
			error?: undefined;
			status?: undefined;
			code?: undefined;
	  }
	| { applicationCredential?: undefined; error: string; status: number; code: string };

// The create/rotate endpoints additionally return the plaintext
// clientSecret -- the only two responses that ever do (list/get/update
// never include it, matching the backend's applicationCredentialResponse
// vs applicationCredentialSecretResponse split).
export type ApplicationCredentialSecretResult =
	| {
			applicationCredential: ApplicationCredentialSummary;
			clientSecret: string;
			error?: undefined;
			status?: undefined;
			code?: undefined;
	  }
	| {
			applicationCredential?: undefined;
			clientSecret?: undefined;
			error: string;
			status: number;
			code: string;
	  };

const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

export async function listApplicationCredentials(
	token: string
): Promise<ApplicationCredentialSummary[]> {
	const response = await fetch(`${API_ORIGIN}/api/application-credentials`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) {
		// Same convention as listGroups: a non-OK response is surfaced as an
		// empty list rather than thrown, logged server-side so the failure
		// stays visible.
		console.error(`listApplicationCredentials: backend returned ${response.status}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.applicationCredentials) ? payload.applicationCredentials : [];
}

// toSecretResult splits the backend's flat
// {id,name,environmentId,scopes,active,clientSecret} response into the
// credential summary plus the one-time plaintext secret.
function toSecretResult(
	payload: unknown,
	fallbackError: string
): ApplicationCredentialSecretResult {
	const p = payload as Partial<ApplicationCredentialSummary & { clientSecret: string }> | null;
	if (!p || typeof p.id !== 'string' || typeof p.clientSecret !== 'string') {
		return { error: fallbackError, code: 'internal', status: 502 };
	}
	return {
		applicationCredential: {
			id: p.id,
			name: p.name ?? '',
			environmentId: p.environmentId ?? '',
			scopes: Array.isArray(p.scopes) ? p.scopes : [],
			active: p.active ?? false
		},
		clientSecret: p.clientSecret
	};
}

export async function createApplicationCredential(
	token: string,
	input: { name: string; environmentId: string; scopes: string[] }
): Promise<ApplicationCredentialSecretResult> {
	const response = await fetch(`${API_ORIGIN}/api/application-credentials`, {
		method: 'POST',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string' ? payload.error : "Couldn't create that credential.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const payload = await response.json().catch(() => null);
	return toSecretResult(payload, "Couldn't create that credential.");
}

export async function updateApplicationCredential(
	token: string,
	id: string,
	input: { name: string; environmentId: string; scopes: string[]; active: boolean }
): Promise<ApplicationCredentialResult> {
	const response = await fetch(
		`${API_ORIGIN}/api/application-credentials/${encodeURIComponent(id)}`,
		{
			method: 'PUT',
			headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
			body: JSON.stringify(input)
		}
	);
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string' ? payload.error : "Couldn't update that credential.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const applicationCredential = await response.json().catch(() => null);
	return applicationCredential
		? { applicationCredential }
		: { error: "Couldn't update that credential.", code: 'internal', status: 502 };
}

export async function rotateApplicationCredential(
	token: string,
	id: string
): Promise<ApplicationCredentialSecretResult> {
	const response = await fetch(
		`${API_ORIGIN}/api/application-credentials/${encodeURIComponent(id)}/rotate`,
		{
			method: 'POST',
			headers: { Authorization: `Bearer ${token}` }
		}
	);
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string'
					? payload.error
					: "Couldn't rotate that credential's secret.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const payload = await response.json().catch(() => null);
	return toSecretResult(payload, "Couldn't rotate that credential's secret.");
}

export async function deleteApplicationCredential(
	token: string,
	id: string
): Promise<{ error: string; status: number; code: string } | null> {
	const response = await fetch(
		`${API_ORIGIN}/api/application-credentials/${encodeURIComponent(id)}`,
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
		error: typeof payload?.error === 'string' ? payload.error : "Couldn't delete that credential.",
		code: typeof payload?.code === 'string' ? payload.code : 'internal',
		status: response.status
	};
}
