import { error } from '@sveltejs/kit';
import { getAuthToken } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { listAuditLog } from '$lib/server/auditLog';
import type { PageServerLoad } from './$types';

// Auth itself is already enforced by dashboard/+layout.server.ts; this load
// only adds the additional permission narrowing on top, the same layering
// dashboard/users/+page.server.ts already uses for its own 403 check.
// Filtering is driven entirely by URL search params and a plain GET form
// (see +page.svelte) rather than a client-side fetch, since this page is
// read-only and SvelteKit's own load re-run already handles that.
export const load: PageServerLoad = async ({ cookies, parent, url }) => {
	const { isAdmin, permissions } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'audits:read')) {
		error(403, 'You do not have permission to view the audit log.');
	}

	const filter = {
		targetType: url.searchParams.get('targetType') ?? '',
		targetId: url.searchParams.get('targetId') ?? '',
		actorId: url.searchParams.get('actorId') ?? ''
	};

	const token = getAuthToken(cookies);
	const entries = token ? await listAuditLog(token, filter) : [];

	return { entries, filter };
};
