<script lang="ts">
	import { tick } from 'svelte';
	import { getLocale, setLocale, locales, type Locale } from '#lib/paraglide/runtime.js';
	import { m } from '#lib/paraglide/messages.js';
	import { theme } from '#lib/theme.svelte.ts';
	import { getInitials } from '#lib/initials.ts';
	import { localeMeta } from './LocaleSwitcher.svelte';

	let { username, isAdmin, collapsed }: { username: string; isAdmin: boolean; collapsed: boolean } =
		$props();

	let open = $state(false);
	let container: HTMLDivElement | undefined = $state();
	let trigger: HTMLButtonElement | undefined = $state();
	let menu: HTMLDivElement | undefined = $state();

	const themeOptions = [
		{ value: 'light', label: m.theme_light, icon: 'icon-[lucide--sun]' },
		{ value: 'dark', label: m.theme_dark, icon: 'icon-[lucide--moon]' },
		{ value: 'system', label: m.theme_system, icon: 'icon-[lucide--monitor]' }
	] as const;
	const selectedTheme = $derived(theme.override ?? 'system');

	function items(): HTMLElement[] {
		return [...(menu?.querySelectorAll<HTMLElement>('[role^="menuitem"]') ?? [])];
	}

	async function toggleOpen() {
		open = !open;
		if (open) {
			await tick();
			items()[0]?.focus();
		}
	}

	function close() {
		open = false;
		trigger?.focus();
	}

	function chooseLocale(locale: Locale) {
		close();
		setLocale(locale);
	}

	function chooseTheme(value: (typeof themeOptions)[number]['value']) {
		theme.setOverride(value === 'system' ? null : value);
		close();
	}

	function handleClickOutside(event: MouseEvent) {
		if (open && container && !container.contains(event.target as Node)) {
			open = false;
		}
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') close();
	}

	function handleMenuKeydown(event: KeyboardEvent) {
		const all = items();
		const current = all.indexOf(document.activeElement as HTMLElement);
		const next = {
			ArrowDown: (current + 1) % all.length,
			ArrowUp: (current - 1 + all.length) % all.length,
			Home: 0,
			End: all.length - 1
		}[event.key];
		if (next === undefined) return;
		event.preventDefault();
		all[next]?.focus();
	}
</script>

<svelte:window onclick={handleClickOutside} onkeydown={handleWindowKeydown} />

<div class="relative border-t border-border" bind:this={container}>
	<button
		type="button"
		class="flex w-full cursor-pointer items-center gap-2.5 px-4 py-3.5 text-left hover:bg-surface-muted"
		aria-haspopup="menu"
		aria-expanded={open}
		aria-label={m.profile_menu_label()}
		bind:this={trigger}
		onclick={toggleOpen}
	>
		<span
			class="flex size-6.5 shrink-0 items-center justify-center rounded-full bg-primary text-[11px] font-semibold text-primary-foreground"
			aria-hidden="true"
		>
			{getInitials(username)}
		</span>
		{#if !collapsed}
			<span class="min-w-0">
				<span class="block truncate text-[12.5px] font-medium text-foreground">{username}</span>
				<span class="block truncate text-[11px] text-muted-foreground">
					{isAdmin ? m.sidebar_role_admin() : m.sidebar_role_member()}
				</span>
			</span>
		{/if}
	</button>

	{#if open}
		<div
			class="absolute z-30 flex min-w-48 flex-col gap-0.5 rounded-xl border border-border bg-surface p-1.5 {collapsed
				? 'bottom-0 left-[calc(100%+0.4rem)]'
				: 'right-2.5 bottom-[calc(100%+0.4rem)] left-2.5'}"
			role="menu"
			tabindex="-1"
			aria-label={m.profile_menu_label()}
			bind:this={menu}
			onkeydown={handleMenuKeydown}
		>
			<p
				class="px-2.5 pt-1 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase"
			>
				{m.locale_switcher_label()}
			</p>
			{#each locales as locale (locale)}
				<button
					type="button"
					class="flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium text-foreground hover:bg-surface-muted {getLocale() ===
					locale
						? 'bg-accent text-accent-foreground'
						: ''}"
					role="menuitemradio"
					aria-checked={getLocale() === locale}
					onclick={() => chooseLocale(locale)}
				>
					<span class="text-lg leading-none" aria-hidden="true">{localeMeta[locale].flag}</span>
					<span>{localeMeta[locale].name}</span>
				</button>
			{/each}

			<p
				class="mt-1 border-t border-border px-2.5 pt-2 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase"
			>
				{m.profile_menu_theme()}
			</p>
			{#each themeOptions as option (option.value)}
				<button
					type="button"
					class="flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium text-foreground hover:bg-surface-muted {selectedTheme ===
					option.value
						? 'bg-accent text-accent-foreground'
						: ''}"
					role="menuitemradio"
					aria-checked={selectedTheme === option.value}
					onclick={() => chooseTheme(option.value)}
				>
					<span class="{option.icon} size-4" aria-hidden="true"></span>
					<span>{option.label()}</span>
				</button>
			{/each}

			<form method="POST" action="/logout" class="mt-1 border-t border-border pt-1">
				<!-- Not closing the menu here: unmounting the form mid-click would cancel the submit -->
				<button
					type="submit"
					class="flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium text-danger hover:bg-surface-muted"
					role="menuitem"
				>
					<span class="icon-[lucide--log-out] size-4" aria-hidden="true"></span>
					<span>{m.logout_button()}</span>
				</button>
			</form>
		</div>
	{/if}
</div>
