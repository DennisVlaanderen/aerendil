<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let isAdmin = $derived(data.isAdmin);
</script>

<svelte:head>
	<title>Dashboard • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-2xl rounded-xl border border-line-1 bg-surface p-6">
		<p class="mb-1 text-xs font-semibold tracking-widest text-nav-active uppercase">
			{isAdmin ? m.dashboard_admin_view() : m.dashboard_member_view()}
		</p>
		<h1 class="text-xl font-semibold text-ink">
			{m.dashboard_welcome({ username: data.username })}
		</h1>
		<p class="mt-2 mb-5 text-ink-muted">{m.dashboard_subtext()}</p>

		<div class="grid grid-cols-[repeat(auto-fit,minmax(12rem,1fr))] gap-4">
			<div class="grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
				<span class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
					>{m.dashboard_access_level()}</span
				>
				<strong class="text-ink"
					>{isAdmin ? m.dashboard_full_admin() : m.dashboard_standard_access()}</strong
				>
			</div>
			<div class="grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
				<span class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
					>{m.dashboard_status()}</span
				>
				<strong class="text-ink"
					>{isAdmin ? m.dashboard_ready_manage() : m.dashboard_ready_work()}</strong
				>
			</div>
		</div>
	</div>
</div>
