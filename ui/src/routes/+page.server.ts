import { redirect } from '@sveltejs/kit';
import { getSession } from '#lib/server/auth.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
	const target = (await getSession(cookies)) ? '/dashboard' : '/login';
	redirect(303, target);
};
