<script lang="ts">
	import { page } from '$app/state';
	import { m } from '#lib/paraglide/messages.js';
	import EnvironmentSwitcher from './EnvironmentSwitcher.svelte';
	import type { EnvironmentSummary } from '#lib/server/environments.ts';

	let {
		environments,
		selectedEnvironmentId
	}: {
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

<header
	class="flex h-16 shrink-0 items-center justify-between border-b border-border bg-background px-7"
>
	<div class="truncate text-base font-semibold text-foreground">{title}</div>

	<EnvironmentSwitcher {environments} {selectedEnvironmentId} />
</header>
