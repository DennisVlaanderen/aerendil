import { error } from '@sveltejs/kit';
import { getAuthToken } from '$lib/server/auth';
import { hasPermission } from '$lib/permissions';
import { listApplicationCredentials } from '$lib/server/applicationCredentials';
import type { PageServerLoad } from './$types';

// Same layering as dashboard/groups/+page.server.ts and
// dashboard/settings/environments/+page.server.ts: auth itself is already
// enforced by dashboard/+layout.server.ts; this load only adds the
// additional permission narrowing on top. Mutations (create/update/delete/
// rotate) live in bff/application-credentials/+server.ts /
// [id]/+server.ts / [id]/rotate/+server.ts instead of form actions, so
// their responses carry real REST status codes.
//
// This whole page is scoped to selectedEnvironmentId -- the same
// environment chosen in the header switcher -- the same way the flags
// sidebar only ever shows flags for the selected environment
// (dashboard/+layout.server.ts). A credential belongs to exactly one
// environment (AC-7.3), so there is never a reason to list, create, or edit
// one against any other environment from this page; `environments` isn't
// re-fetched here at all -- `data.environments` falls through from the
// dashboard layout's session-scoped list, which is all a name lookup for
// the (already access-checked) selected environment needs.
//
// listApplicationCredentials is called with selectedEnvironmentId itself
// (mirroring listFlags) so the backend does the filtering -- the backend
// route requires and enforces access to that environmentId (see
// applicationCredentialsGetHandler), rather than this load fetching every
// credential across every environment and filtering client-side.
export const load: PageServerLoad = async ({ cookies, parent }) => {
	const { isAdmin, permissions, selectedEnvironmentId } = await parent();
	if (!hasPermission({ isAdmin, permissions }, 'applicationCredentials:read')) {
		error(403, 'You do not have permission to view application credentials.');
	}

	const token = getAuthToken(cookies);
	const applicationCredentials =
		token && selectedEnvironmentId
			? await listApplicationCredentials(token, selectedEnvironmentId)
			: [];

	return { applicationCredentials, selectedEnvironmentId };
};
