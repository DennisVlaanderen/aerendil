// Thin fetch wrapper for the browser-side calls that replaced SvelteKit form
// actions (see the users/groups/login +server.ts endpoints) -- form actions
// always respond 200 on the wire with the real status embedded in the JSON
// body (SvelteKit's own use:enhance protocol), which isn't real REST. These
// +server.ts endpoints return the actual HTTP status, so callers here can
// rely on response.ok/response.status directly instead of an embedded field.
export type ApiResult<T> =
	| { data: T; error?: undefined; code?: undefined }
	| { data?: undefined; error: string; code?: string };

export async function apiRequest<T>(url: string, init: RequestInit = {}): Promise<ApiResult<T>> {
	const response = await fetch(url, {
		...init,
		headers: { 'content-type': 'application/json', ...init.headers }
	});

	const payload = await response.json().catch(() => null);
	if (!response.ok) {
		return {
			error: typeof payload?.error === 'string' ? payload.error : 'Something went wrong.',
			code: typeof payload?.code === 'string' ? payload.code : undefined
		};
	}
	return { data: payload as T };
}
