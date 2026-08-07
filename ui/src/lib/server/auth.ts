import { env } from '$env/dynamic/private';
import { dev } from '$app/environment';
import type { Cookies } from '@sveltejs/kit';
import type { EnvironmentSummary } from './environments';

export interface Session {
	id: string;
	username: string;
	isAdmin: boolean;
	permissions: string[];
	// The environments this session can actually use -- Admin gets every
	// environment, everyone else only what their groups grant (see the
	// backend's api.resolveEnvironmentSummaries). Resolved to full
	// {id, name, order} records by /api/auth/me itself, not left as bare
	// IDs, so no caller needs environments:read just to learn the names of
	// environments it already has access to.
	environments: EnvironmentSummary[];
	// Derived from environments -- kept alongside it so callers doing a
	// pure access check (e.g. lib/permissions.ts's hasEnvironmentAccess)
	// don't need to map over environment objects themselves.
	environmentIds: string[];
}

const AUTH_COOKIE = 'aerendil.auth';
const SELECTED_ENV_COOKIE = 'aerendil.selected-environment';
const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

function parseSession(payload: unknown): Session | null {
	if (typeof payload !== 'object' || payload === null) {
		return null;
	}
	const { user, isAdmin, permissions, environments } = payload as {
		user?: unknown;
		isAdmin?: unknown;
		permissions?: unknown;
		environments?: unknown;
	};
	if (typeof user !== 'object' || user === null) {
		return null;
	}
	const { id, username } = user as { id?: unknown; username?: unknown };
	if (
		typeof id !== 'string' ||
		typeof username !== 'string' ||
		typeof isAdmin !== 'boolean' ||
		!Array.isArray(permissions) ||
		!permissions.every((p) => typeof p === 'string') ||
		!Array.isArray(environments) ||
		!environments.every(
			(e) =>
				typeof e === 'object' &&
				e !== null &&
				typeof e.id === 'string' &&
				typeof e.name === 'string' &&
				typeof e.order === 'number'
		)
	) {
		return null;
	}
	const typedEnvironments = environments as EnvironmentSummary[];
	return {
		id,
		username,
		isAdmin,
		permissions,
		environments: typedEnvironments,
		environmentIds: typedEnvironments.map((e) => e.id)
	};
}

// Returns null only when the backend couldn't be reached at all (network
// failure) -- once it responds, even a failure carries the real status and
// error code back to the caller (see bff/login/+server.ts) instead of
// collapsing every failure mode into the same generic null.
export async function login(
	username: string,
	password: string
): Promise<{ token: string } | { error: string; code: string; status: number } | null> {
	const response = await fetch(`${API_ORIGIN}/api/auth/login`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ username, password })
	}).catch(() => null);
	if (!response) {
		return null;
	}

	const payload = await response.json().catch(() => null);
	if (!response.ok || typeof payload?.token !== 'string' || !payload.token) {
		return {
			error: typeof payload?.error === 'string' ? payload.error : 'Invalid username or password.',
			code: typeof payload?.code === 'string' ? payload.code : 'internal',
			status: response.status
		};
	}

	return { token: payload.token };
}

// getSession always re-fetches /api/auth/me rather than trusting any local
// cache -- the backend resolves permissions fresh from the store on every
// call, so a group/permission change or deactivation is reflected on the
// user's very next request, with no token revocation infrastructure needed.
export async function getSession(cookies: Cookies): Promise<Session | null> {
	const token = cookies.get(AUTH_COOKIE);
	if (!token) {
		return null;
	}

	const response = await fetch(`${API_ORIGIN}/api/auth/me`, {
		headers: { Authorization: `Bearer ${token}` }
	}).catch(() => null);
	if (!response?.ok) {
		return null;
	}

	return parseSession(await response.json().catch(() => null));
}

export function setAuthCookie(cookies: Cookies, token: string) {
	cookies.set(AUTH_COOKIE, token, {
		path: '/',
		httpOnly: true,
		sameSite: 'lax',
		secure: !dev,
		maxAge: 60 * 60 * 24
	});
}

export function clearAuthCookie(cookies: Cookies) {
	cookies.delete(AUTH_COOKIE, { path: '/' });
}

export function getAuthToken(cookies: Cookies): string | null {
	return cookies.get(AUTH_COOKIE) ?? null;
}

// The selected environment is a UI preference, not a security boundary --
// the bff/selected-environment route still re-validates the posted ID
// against the caller's session-visible environmentIds before storing it, so
// an httpOnly cookie here is just consistency with AUTH_COOKIE's pattern,
// not a trust mechanism.
export function setSelectedEnvironmentCookie(cookies: Cookies, environmentId: string) {
	cookies.set(SELECTED_ENV_COOKIE, environmentId, {
		path: '/',
		httpOnly: true,
		sameSite: 'lax',
		secure: !dev,
		maxAge: 60 * 60 * 24 * 365
	});
}

export function getSelectedEnvironmentId(cookies: Cookies): string | null {
	return cookies.get(SELECTED_ENV_COOKIE) ?? null;
}
