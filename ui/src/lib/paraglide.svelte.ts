import type { Locale as _Locale } from '#lib/paraglide/runtime.js';
import { browser } from '$app/env';
import { goto } from '$app/navigation';
import { page } from '$app/state';

import {
	baseLocale,
	localizeUrl,
	overwriteGetLocale,
	overwriteSetLocale,
	toLocale
} from '#lib/paraglide/runtime.js';

export class Locale {
	#current: _Locale = $state(
		toLocale(browser && document.querySelector('html')?.lang) ?? baseLocale
	);

	constructor() {
		overwriteGetLocale(() => this.#current);

		overwriteSetLocale((locale) => {
			this.#current = locale;
			// Target is the current (already-resolved) route re-localized by
			// paraglide's own localizeUrl(), not a literal route id resolve() could
			// type-check.
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(localizeUrl(page.url.pathname, { locale }).href);
		});
	}
}
