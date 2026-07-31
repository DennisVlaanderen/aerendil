import { env } from '$env/dynamic/private';

export interface GroupSummary {
	id: string;
	name: string;
	permissions: string[];
	system: boolean;
}

// Mirrors backend/internal/auth.AllPermissions -- the catalog a Groups
// permission picker offers. Kept in sync by hand since there's no
// dedicated endpoint to fetch it from; the backend independently rejects
// any unknown permission string regardless of what the UI offers.
export const ALL_PERMISSIONS = [
	'flags:read',
	'flags:write',
	'users:read',
	'users:create',
	'users:update',
	'users:delete',
	'groups:read',
	'groups:create',
	'groups:update',
	'groups:delete',
	'audits:read',
	'environments:read',
	'environments:create',
	'environments:update',
	'environments:delete'
] as const;

// `status` mirrors the backend's own HTTP status verbatim -- see the
// identical convention on UserResult (lib/server/users.ts) for why.
export type GroupResult =
	| { group: GroupSummary; error?: undefined; status?: undefined }
	| { group?: undefined; error: string; status: number };

const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

export async function listGroups(token: string): Promise<GroupSummary[]> {
	const response = await fetch(`${API_ORIGIN}/api/groups`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) {
		// A non-OK response (403 missing groups:read, 401, 500, ...) is
		// deliberately still surfaced to the caller as an empty list rather
		// than thrown -- group data is often optional context for another
		// page (e.g. populating checkboxes on the Users page), and a caller
		// lacking groups:read should still be able to use that page. Logging
		// here at least keeps the failure visible server-side instead of
		// being indistinguishable from a genuinely empty list.
		console.error(`listGroups: backend returned ${response.status}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.groups) ? payload.groups : [];
}

export async function createGroup(
	token: string,
	input: { name: string; permissions: string[] }
): Promise<GroupResult> {
	const response = await fetch(`${API_ORIGIN}/api/groups`, {
		method: 'POST',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error: typeof payload?.error === 'string' ? payload.error : "Couldn't create that group.",
			status: response.status
		};
	}

	const group = await response.json().catch(() => null);
	return group ? { group } : { error: "Couldn't create that group.", status: 502 };
}

export async function updateGroup(
	token: string,
	id: string,
	input: { name: string; permissions: string[] }
): Promise<GroupResult> {
	const response = await fetch(`${API_ORIGIN}/api/groups/${encodeURIComponent(id)}`, {
		method: 'PUT',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error: typeof payload?.error === 'string' ? payload.error : "Couldn't update that group.",
			status: response.status
		};
	}

	const group = await response.json().catch(() => null);
	return group ? { group } : { error: "Couldn't update that group.", status: 502 };
}

export async function deleteGroup(
	token: string,
	id: string
): Promise<{ error: string; status: number } | null> {
	const response = await fetch(`${API_ORIGIN}/api/groups/${encodeURIComponent(id)}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});
	if (response.ok) {
		return null;
	}

	const payload = await response.json().catch(() => null);
	return {
		error: typeof payload?.error === 'string' ? payload.error : "Couldn't delete that group.",
		status: response.status
	};
}
