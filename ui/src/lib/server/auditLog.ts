import { env } from '$env/dynamic/private';

export interface AuditEntry {
	id: number;
	timestamp: number;
	actorId: string;
	actorType?: string;
	action: string;
	targetType: string;
	targetId?: string;
	before?: string;
	after?: string;
	success: boolean;
	statusCode: number;
	error?: string;
}

export interface AuditLogFilter {
	targetType?: string;
	targetId?: string;
	actorId?: string;
}

const API_ORIGIN = env.AERENDIL_API_ORIGIN?.trim() || 'http://127.0.0.1:8080';

export async function listAuditLog(token: string, filter: AuditLogFilter = {}): Promise<AuditEntry[]> {
	const params = new URLSearchParams();
	if (filter.targetType) params.set('targetType', filter.targetType);
	if (filter.targetId) params.set('targetId', filter.targetId);
	if (filter.actorId) params.set('actorId', filter.actorId);
	const query = params.toString();

	const response = await fetch(`${API_ORIGIN}/api/audits${query ? `?${query}` : ''}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) {
		// See the identical comment in lib/server/groups.ts's listGroups.
		console.error(`listAuditLog: backend returned ${response.status}`);
		return [];
	}

	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.audits) ? payload.audits : [];
}
