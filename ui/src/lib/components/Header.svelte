<script lang="ts">
	import { page } from '$app/state';
	import { m } from '$lib/paraglide/messages.js';
	import { theme } from '$lib/theme.svelte';
	import { getInitials } from '$lib/initials';
	import LocaleSwitcher from './LocaleSwitcher.svelte';
	import EnvironmentSwitcher from './EnvironmentSwitcher.svelte';
	import type { EnvironmentSummary } from '$lib/server/environments';

	let {
		username,
		environments,
		selectedEnvironmentId
	}: {
		username: string;
		environments: EnvironmentSummary[];
		selectedEnvironmentId: string | undefined;
	} = $props();

	function resolveTitle(pathname: string, data: Record<string, unknown>) {
		if (pathname.startsWith('/dashboard/flags/')) {
			const flag = data.flag as { key: string } | undefined;
			return flag?.key ?? m.nav_flags();
		}
		if (pathname.startsWith('/dashboard/users/')) {
			const user = data.user as { username: string } | undefined;
			return user?.username ?? m.nav_users();
		}
		if (pathname.startsWith('/dashboard/users')) return m.nav_users();
		if (pathname.startsWith('/dashboard/groups')) return m.nav_groups();
		if (pathname.startsWith('/dashboard/audit-log')) return m.nav_audit_log();
		return m.nav_dashboard();
	}

	let title = $derived(resolveTitle(page.url.pathname, page.data));
</script>

<header class="flex h-16 shrink-0 items-center justify-between border-b border-line-2 bg-page px-7">
	<div class="truncate text-base font-semibold text-ink">{title}</div>

	<div class="flex items-center gap-4">
		<EnvironmentSwitcher {environments} {selectedEnvironmentId} />

		<button
			type="button"
			class="flex size-8 cursor-pointer items-center justify-center rounded-lg bg-control text-ink-muted"
			aria-label={theme.effective === 'dark' ? m.theme_toggle_light() : m.theme_toggle_dark()}
			onclick={() => theme.toggle()}
		>
			{#if theme.effective === 'dark'}
				<span class="icon-[lucide--sun] size-4" aria-hidden="true"></span>
			{:else}
				<span class="icon-[lucide--moon] size-4" aria-hidden="true"></span>
			{/if}
		</button>

		<LocaleSwitcher compact={false} />

		<div
			class="flex size-7.5 shrink-0 items-center justify-center rounded-full bg-avatar text-xs font-semibold text-cream"
			aria-hidden="true"
		>
			{getInitials(username)}
		</div>
	</div>
</header>
