import { browser } from '$app/env';

const STORAGE_KEY = 'aerendil-theme';

type Override = 'light' | 'dark' | null;

function readStoredOverride(): Override {
	const raw = localStorage.getItem(STORAGE_KEY);
	return raw === 'light' || raw === 'dark' ? raw : null;
}

class ThemeStore {
	// Seeded from the same localStorage/matchMedia logic the inline script in
	// app.html already ran pre-hydration, so this never disagrees with the
	// <html class="dark"> the page was first painted with.
	override: Override = $state(browser ? readStoredOverride() : null);
	#systemDark = $state(browser ? matchMedia('(prefers-color-scheme: dark)').matches : false);

	effective: 'light' | 'dark' = $derived(this.override ?? (this.#systemDark ? 'dark' : 'light'));

	constructor() {
		if (!browser) return;
		matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (event) => {
			this.#systemDark = event.matches;
			this.#applyDom();
		});
	}

	#applyDom() {
		document.documentElement.classList.toggle('dark', this.effective === 'dark');
	}

	setOverride(value: Override) {
		this.override = value;
		if (value) {
			localStorage.setItem(STORAGE_KEY, value);
		} else {
			localStorage.removeItem(STORAGE_KEY);
		}
		this.#applyDom();
	}

	toggle() {
		this.setOverride(this.effective === 'dark' ? 'light' : 'dark');
	}
}

export const theme = new ThemeStore();
