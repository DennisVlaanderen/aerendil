import { redirect } from '@sveltejs/kit';
import { clearAuthCookie } from '#lib/server/auth.ts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ cookies }) => {
	clearAuthCookie(cookies);
	redirect(303, '/login');
};
