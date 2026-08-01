<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// data.environments/selectedEnvironmentId cascade down from
	// dashboard/+layout.server.ts -- the same key can now exist
	// independently per environment, so the detail view needs to say which
	// one it's showing.
	let selectedEnvironmentName = $derived(
		data.environments.find((e) => e.id === data.selectedEnvironmentId)?.name
	);
</script>

<svelte:head>
	<title>{data.flag.key} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-2xl rounded-xl border border-line-1 bg-surface p-6">
		<p class="mb-1 text-xs font-semibold tracking-widest text-nav-active uppercase">
			{m.nav_flags()}
		</p>
		<h1 class="mb-1 flex items-center gap-3 text-xl font-semibold break-words text-ink">
			<span class="icon-[lucide--flag] size-6 shrink-0 text-gold" aria-hidden="true"></span>
			{data.flag.key}
		</h1>
		<p class="mb-5 text-sm text-ink-muted">
			{selectedEnvironmentName ? m.flag_detail_environment({ name: selectedEnvironmentName }) : ''}
		</p>

		<div class="grid grid-cols-[repeat(auto-fit,minmax(12rem,1fr))] gap-4">
			<div class="grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
				<span class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
					>{m.flag_detail_status()}</span
				>
				<strong class="flex items-center gap-2 {data.flag.enabled ? 'text-success' : 'text-ink'}">
					{#if data.flag.enabled}
						<span class="icon-[lucide--toggle-right] size-4 shrink-0" aria-hidden="true"></span>
					{:else}
						<span class="icon-[lucide--toggle-left] size-4 shrink-0" aria-hidden="true"></span>
					{/if}
					{data.flag.enabled ? m.flag_enabled() : m.flag_disabled()}
				</strong>
			</div>
			<div class="grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
				<span class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
					>{m.flag_detail_version()}</span
				>
				<strong class="text-ink">{data.flag.version}</strong>
			</div>
			<div class="col-span-full grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
				<span class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
					>{m.flag_detail_value()}</span
				>
				<strong class="text-ink">{data.flag.value || '—'}</strong>
			</div>
		</div>
	</div>
</div>
