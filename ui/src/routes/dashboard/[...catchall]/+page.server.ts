import { redirect } from '@sveltejs/kit';
import { setFlash } from '#lib/server/flash.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = ({ cookies }) => {
	setFlash(cookies, 'route-not-found');
	redirect(307, '/dashboard');
};
