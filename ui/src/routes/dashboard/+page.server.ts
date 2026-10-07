import { readFlash } from '#lib/server/flash.ts';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = ({ cookies }) => {
	return { flashReason: readFlash(cookies) };
};
