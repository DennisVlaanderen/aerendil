import { env } from '$env/dynamic/private';

export interface UserSummary {
	id: string;
	username: string;
	groupIds: string[];
	active: boolean;
}

// `status` mirrors the backend's own HTTP status verbatim -- the
// dashboard/users +server.ts routes proxy it straight through rather than
// re-deriving their own, so the real REST status the Go API chose (403 for
// store.ErrLastAdmin, 409 for a taken username, etc.) is what ends up on the
// wire, not a guess reconstructed from the error text.
export type UserResult =
	| { user: UserSummary; error?: undefined; status?: undefined; code?: undefined }
	| { user?: undefined; error: string; status: number; code: string };

const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

export async function listUsers(token: string): Promise<UserSummary[]> {
	const response = await fetch(`${API_ORIGIN}/api/users`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) {
		// See the identical comment in lib/server/groups.ts's listGroups.
		console.error(`listUsers: backend returned ${response.status}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.users) ? payload.users : [];
}

export async function createUser(
	token: string,
	input: { username: string; password: string; groupIds: string[] }
): Promise<UserResult> {
	const response = await fetch(`${API_ORIGIN}/api/users`, {
		method: 'POST',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string'
					? payload.error
					: "Couldn't create that user. The username may already be taken.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const user = await response.json().catch(() => null);
	return user ? { user } : { error: "Couldn't create that user.", code: 'internal', status: 502 };
}

// An empty password means "leave it unchanged" -- mirrors the backend's own
// usersPutHandler semantics (see backend/internal/api/users.go), so the edit
// form never needs to round-trip the existing password hash. groupIds is
// similarly optional: omitting it (rather than sending []) tells the
// backend to leave the user's existing group membership unchanged, which
// matters when the caller can't see the full group list to specify a real
// value in the first place.
export async function updateUser(
	token: string,
	id: string,
	input: { username: string; password: string; groupIds?: string[]; active: boolean }
): Promise<UserResult> {
	const response = await fetch(`${API_ORIGIN}/api/users/${encodeURIComponent(id)}`, {
		method: 'PUT',
		headers: { 'content-type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const payload = await response.json().catch(() => null);
		return {
			error:
				typeof payload?.error === 'string'
					? payload.error
					: "Couldn't update that user. The username may already be taken.",
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	const user = await response.json().catch(() => null);
	return user ? { user } : { error: "Couldn't update that user.", code: 'internal', status: 502 };
}

// Returns null on success, or the backend's real status alongside a
// user-facing error message on failure -- e.g. rejecting deletion of the
// last remaining admin account (see store.ErrLastAdmin) is a 403 the
// backend itself chose, not a status this layer should reinvent.
export async function deleteUser(
	token: string,
	id: string
): Promise<{ error: string; status: number; code: string } | null> {
	const response = await fetch(`${API_ORIGIN}/api/users/${encodeURIComponent(id)}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});
	if (response.ok) {
		return null;
	}

	const payload = await response.json().catch(() => null);
	return {
		error: typeof payload?.error === 'string' ? payload.error : "Couldn't delete that user.",
		code: typeof payload?.code === 'string' ? payload.code : 'internal',
		status: response.status
	};
}
