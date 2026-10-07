import type { ClientInit } from '@sveltejs/kit/hooks';
import { Locale } from '#lib/paraglide.svelte.ts';

export const init: ClientInit = () => {
	new Locale();
};
