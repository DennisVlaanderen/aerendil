<script lang="ts">
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { hasPermission } from '#lib/permissions.ts';
	import { segmentUsage } from '#lib/flags.ts';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const canCreate = $derived(hasPermission(data, 'segments:write'));
</script>

<svelte:head>
	<title>{m.segments_page_title()} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-4xl rounded-xl border border-border bg-surface p-6">
		<div class="mb-5 flex items-center justify-between gap-3">
			<div>
				<h1 class="mb-1 text-xl font-semibold text-foreground">{m.segments_page_title()}</h1>
				<p class="text-muted-foreground">{m.segments_page_subtitle()}</p>
			</div>
			{#if canCreate}
				<a
					href={localizedResolve('/dashboard/segments/new')}
					class="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground no-underline hover:bg-primary-hover"
				>
					{m.segment_create_page_title()}
				</a>
			{/if}
		</div>

		{#if data.segments.length === 0}
			<p class="text-sm text-muted-foreground">{m.segments_empty()}</p>
		{:else}
			<table class="w-full text-left text-sm">
				<thead class="text-xs tracking-wider text-muted-foreground uppercase">
					<tr>
						<th class="py-2 font-semibold">{m.segment_key_label()}</th>
						<th class="py-2 font-semibold">{m.segment_name_label()}</th>
						<th class="py-2 font-semibold">{m.segment_used_by()}</th>
					</tr>
				</thead>
				<tbody>
					{#each data.segments as segment (segment.key)}
						<tr class="border-t border-border">
							<td class="py-2 font-mono">
								<a
									class="text-primary hover:underline"
									href={localizedResolve(`/dashboard/segments/${encodeURIComponent(segment.key)}`)}
								>
									{segment.key}
								</a>
							</td>
							<td class="py-2 text-foreground">{segment.name}</td>
							<td class="py-2 text-muted-foreground">
								{segmentUsage(data.flags, segment.key).join(', ') || '—'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</div>
