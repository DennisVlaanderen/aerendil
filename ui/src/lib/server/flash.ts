import { dev } from '$app/env';
import type { Cookies } from '@sveltejs/kit';

export type FlashReason = 'route-not-found';

const FLASH_COOKIE = 'aerendil.flash';

export function setFlash(cookies: Cookies, reason: FlashReason) {
	cookies.set(FLASH_COOKIE, reason, {
		path: '/',
		httpOnly: true,
		sameSite: 'lax',
		secure: !dev,
		maxAge: 60
	});
}

export function readFlash(cookies: Cookies): FlashReason | null {
	const reason = cookies.get(FLASH_COOKIE);
	if (reason) {
		cookies.delete(FLASH_COOKIE, { path: '/' });
	}
	return (reason as FlashReason) ?? null;
}
