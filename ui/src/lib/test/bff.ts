import { vi } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
import type { Session } from '#lib/server/auth.ts';

export function jsonResponse(status: number, body: unknown): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		json: async () => body
	} as Response;
}

export function fakeCookies(initial: Record<string, string> = {}): Cookies {
	const store: Record<string, string> = { ...initial };
	return {
		get: vi.fn((name: string) => store[name]),
		getAll: vi.fn(() => Object.entries(store).map(([name, value]) => ({ name, value }))),
		set: vi.fn((name: string, value: string) => {
			store[name] = value;
		}),
		delete: vi.fn((name: string) => {
			delete store[name];
		}),
		serialize: vi.fn((name: string, value: string) => `${name}=${value}`)
	} as unknown as Cookies;
}

export function makeSession(overrides: Partial<Session> = {}): Session {
	const environments = overrides.environments ?? [
		{ id: 'env-prod', name: 'Production', order: 0, color: '#ef4444' }
	];
	return {
		id: 'u-1',
		username: 'alice',
		isAdmin: false,
		permissions: [],
		environments,
		environmentIds: overrides.environmentIds ?? environments.map((env) => env.id),
		...overrides
	};
}

export function jsonRequest(body: unknown, method = 'POST', url = 'http://localhost/bff'): Request {
	return new Request(url, {
		method,
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export function invalidJsonRequest(method = 'POST', url = 'http://localhost/bff'): Request {
	return new Request(url, {
		method,
		headers: { 'content-type': 'application/json' },
		body: '{invalid-json'
	});
}

export interface FakeRequestEventOptions {
	request?: Request;
	cookies?: Cookies;
	params?: Record<string, string>;
	url?: string | URL;
}

export function makeRequestEvent<Event>(options: FakeRequestEventOptions = {}): Event {
	const resolvedUrl =
		typeof options.url === 'string'
			? new URL(options.url)
			: (options.url ?? new URL(options.request?.url ?? 'http://localhost/bff'));

	return {
		request: options.request ?? new Request(resolvedUrl, { method: 'GET' }),
		cookies: options.cookies ?? fakeCookies({ 'aerendil.auth': 'tok-123' }),
		params: options.params ?? {},
		url: resolvedUrl,
		fetch: globalThis.fetch,
		getClientAddress: () => '127.0.0.1',
		locals: {},
		platform: undefined,
		route: { id: null },
		isDataRequest: false,
		isSubRequest: false,
		isRemoteRequest: false,
		setHeaders: vi.fn(),
		tracing: { enabled: false, root: {}, current: {} }
	} as unknown as Event;
}
