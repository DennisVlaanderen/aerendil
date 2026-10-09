import { AERENDIL_API_ORIGIN as API_ORIGIN } from '$app/env/private';

export interface AuditEntry {
	id: number;
	timestamp: number;
	actorId: string;
	actorType?: string;
	action: string;
	targetType: string;
	targetId?: string;
	// Nested JSON; a string if the snapshot wasn't JSON.
	before?: unknown;
	after?: unknown;
	success: boolean;
	statusCode: number;
	error?: string;
}

export interface AuditLogFilter {
	targetType?: string;
	targetId?: string;
	actorId?: string;
	// RFC 3339, inclusive.
	from?: string;
	to?: string;
	limit?: number;
	cursor?: string;
}

// Mirrors the backend's auditPage.
export interface AuditLogPage {
	limit: number;
	total: number;
	start: number;
	end: number;
	prevCursor?: string;
	nextCursor?: string;
}

export interface AuditLogResult {
	entries: AuditEntry[];
	page: AuditLogPage;
}

export async function listAuditLog(
	token: string,
	filter: AuditLogFilter = {}
): Promise<AuditLogResult> {
	const empty = { entries: [], page: { limit: filter.limit ?? 25, total: 0, start: 0, end: 0 } };
	const params = new URLSearchParams();
	if (filter.targetType) params.set('targetType', filter.targetType);
	if (filter.targetId) params.set('targetId', filter.targetId);
	if (filter.actorId) params.set('actorId', filter.actorId);
	if (filter.from) params.set('from', filter.from);
	if (filter.to) params.set('to', filter.to);
	if (filter.limit) params.set('limit', String(filter.limit));
	if (filter.cursor) params.set('cursor', filter.cursor);
	const query = params.toString();

	const response = await fetch(`${API_ORIGIN}/api/audits${query ? `?${query}` : ''}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) {
		// See the identical comment in lib/server/groups.ts's listGroups.
		console.error(`listAuditLog: backend returned ${response.status}`);
		return empty;
	}

	const payload = await response.json().catch(() => null);
	if (!Array.isArray(payload?.audits) || !payload.page) return empty;
	return { entries: payload.audits, page: payload.page };
}
