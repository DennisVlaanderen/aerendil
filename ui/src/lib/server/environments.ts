import { env } from '$env/dynamic/private';

export interface EnvironmentSummary {
	id: string;
	name: string;
	order: number;
}

// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on GroupResult (lib/server/groups.ts) for why.
export type EnvironmentResult =
	| { environment: EnvironmentSummary; error?: undefined; status?: undefined; code?: undefined }
	| { environment?: undefined; error: string; status: number; code: string };

const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

export async function listEnvironments(token: string): Promise<EnvironmentSummary[]> {
	const response = await fetch(`${API_ORIGIN}/api/environments`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) {
		// A non-OK response (403 missing environments:read, 401, 500, ...) is
		// deliberately still surfaced to the caller as an empty list rather
		// than thrown -- mirrors listGroups' identical convention.
		console.error(`listEnvironments: backend returned ${response.status}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.environments) ? payload.environments : [];
}

export async function createEnvironment(
	token: string,
	input: { name: string }
): Promise<EnvironmentResult> {
	const response = await fetch(`${API_ORIGIN}/api/environments`, {
		method: 'POST',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string' ? payload.error : "Couldn't create that environment.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const environment = await response.json().catch(() => null);
	return environment
		? { environment }
		: { error: "Couldn't create that environment.", code: 'internal', status: 502 };
}

export async function updateEnvironment(
	token: string,
	id: string,
	input: { name: string }
): Promise<EnvironmentResult> {
	const response = await fetch(`${API_ORIGIN}/api/environments/${encodeURIComponent(id)}`, {
		method: 'PUT',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string' ? payload.error : "Couldn't update that environment.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const environment = await response.json().catch(() => null);
	return environment
		? { environment }
		: { error: "Couldn't update that environment.", code: 'internal', status: 502 };
}

export async function deleteEnvironment(
	token: string,
	id: string
): Promise<{ error: string; status: number; code: string } | null> {
	const response = await fetch(`${API_ORIGIN}/api/environments/${encodeURIComponent(id)}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});
	if (response.ok) {
		return null;
	}

	const payload = await response.json().catch(() => null);
	return {
		error: typeof payload?.error === 'string' ? payload.error : "Couldn't delete that environment.",
		code: typeof payload?.code === 'string' ? payload.code : 'internal',
		status: response.status
	};
}
