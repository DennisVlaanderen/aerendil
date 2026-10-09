<script lang="ts">
	import { getLocale, setLocale, locales, type Locale } from '#lib/paraglide/runtime.js';
	import { m } from '#lib/paraglide/messages.js';

	let { compact = false }: { compact?: boolean } = $props();

	const localeMeta: Record<Locale, { name: string; flag: string }> = {
		en: { name: 'English', flag: '🇬🇧' },
		'nl-nl': { name: 'Nederlands', flag: '🇳🇱' }
	};

	let open = $state(false);
	let openUpward = $state(false);
	let container: HTMLDivElement | undefined = $state();

	const MENU_ROW_HEIGHT = 40;
	const MENU_PADDING = 16;

	function toggleOpen() {
		if (!open && container) {
			const rect = container.getBoundingClientRect();
			const estimatedMenuHeight = locales.length * MENU_ROW_HEIGHT + MENU_PADDING;
			const spaceBelow = window.innerHeight - rect.bottom;
			const spaceAbove = rect.top;
			openUpward = spaceBelow < estimatedMenuHeight && spaceAbove > spaceBelow;
		}
		open = !open;
	}

	function choose(locale: Locale) {
		setLocale(locale);
		open = false;
	}

	function handleClickOutside(event: MouseEvent) {
		if (open && container && !container.contains(event.target as Node)) {
			open = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			open = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} onkeydown={handleKeydown} />

<div class="relative flex {compact ? 'justify-center' : ''}" bind:this={container}>
	<button
		type="button"
		class="flex cursor-pointer items-center gap-2 rounded-lg border border-border bg-background text-[13.5px] font-medium whitespace-nowrap text-foreground {compact
			? 'gap-1 p-2'
			: 'px-2.5 py-2.25'}"
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-label={m.locale_switcher_label()}
		onclick={toggleOpen}
	>
		<span class="text-lg leading-none" aria-hidden="true">{localeMeta[getLocale()].flag}</span>
		{#if !compact}<span>{localeMeta[getLocale()].name}</span>{/if}
		<span
			class="icon-[lucide--chevron-down] size-3.5 text-muted-foreground transition-transform duration-150 {open
				? 'rotate-180'
				: ''}"
			aria-hidden="true"
		></span>
	</button>

	{#if open}
		<ul
			class="absolute z-30 flex min-w-40 flex-col gap-0.5 rounded-xl border border-border bg-surface p-1.5 {openUpward
				? 'bottom-[calc(100%+0.4rem)]'
				: 'top-[calc(100%+0.4rem)]'} {compact ? 'left-0' : 'right-0'}"
			role="listbox"
			aria-label={m.locale_switcher_label()}
		>
			{#each locales as locale (locale)}
				<li>
					<button
						type="button"
						class="flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium text-foreground hover:bg-surface-muted {getLocale() ===
						locale
							? 'bg-accent text-accent-foreground'
							: ''}"
						role="option"
						aria-selected={getLocale() === locale}
						onclick={() => choose(locale)}
					>
						<span class="text-lg leading-none" aria-hidden="true">{localeMeta[locale].flag}</span>
						<span>{localeMeta[locale].name}</span>
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
