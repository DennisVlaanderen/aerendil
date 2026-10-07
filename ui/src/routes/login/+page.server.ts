import { redirect } from '@sveltejs/kit';
import { getSession } from '#lib/server/auth.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
	if (await getSession(cookies)) {
		redirect(303, '/dashboard');
	}
};
