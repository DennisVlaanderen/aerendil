<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Raft only elects a leader with a voter majority, so no leader means no quorum.
	let lacksQuorum = $derived(data.status !== null && !data.status.leaderId);
</script>

<svelte:head>
	<title>{m.cluster_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-foreground">{m.cluster_page_title()}</h1>
		<p class="mt-1 text-muted-foreground">{m.cluster_page_subtitle()}</p>
	</div>

	{#if data.status === null}
		<p role="alert" class="rounded-xl bg-danger/10 p-5 text-sm text-danger">
			{m.cluster_load_error()}
		</p>
	{:else}
		{#if lacksQuorum}
			<div role="alert" class="flex items-center gap-3 rounded-xl bg-danger/10 p-5 text-danger">
				<span class="icon-[lucide--triangle-alert] size-5 shrink-0" aria-hidden="true"></span>
				<div>
					<p class="font-semibold">{m.cluster_no_quorum_title()}</p>
					<p class="text-sm">{m.cluster_no_quorum_body()}</p>
				</div>
			</div>
		{/if}

		<dl class="grid gap-4 sm:grid-cols-4">
			{#each [[m.cluster_leader(), data.status.leaderId || '—'], [m.cluster_term(), data.status.term], [m.cluster_commit_index(), data.status.commitIndex], [m.cluster_last_log_index(), data.status.lastLogIndex]] as [label, value] (label)}
				<div class="rounded-xl border border-border bg-surface p-5">
					<dt class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
						{label}
					</dt>
					<dd class="mt-1 truncate text-lg font-semibold text-foreground">{value}</dd>
				</div>
			{/each}
		</dl>

		<div class="overflow-x-auto rounded-xl border border-border bg-surface">
			<table class="w-full text-left text-sm">
				<thead>
					<tr
						class="border-b border-border text-xs font-semibold tracking-wide text-muted-foreground uppercase"
					>
						<th class="px-3 py-3">{m.cluster_table_node()}</th>
						<th class="px-3 py-3">{m.cluster_table_address()}</th>
						<th class="px-3 py-3">{m.cluster_table_suffrage()}</th>
						<th class="px-3 py-3">{m.cluster_table_role()}</th>
					</tr>
				</thead>
				<tbody>
					{#each data.status.nodes as node, i (node.id)}
						<tr class={i > 0 ? 'border-t border-border' : ''}>
							<td class="px-3 py-3 font-medium text-foreground">{node.id}</td>
							<td class="px-3 py-3 text-foreground">{node.address}</td>
							<td class="px-3 py-3 text-foreground">{node.suffrage}</td>
							<td class="px-3 py-3">
								<span
									class="rounded-full px-2 py-0.5 text-xs font-medium {node.leader
										? 'bg-success/10 text-success'
										: 'text-muted-foreground'}"
								>
									{node.leader ? m.cluster_role_leader() : m.cluster_role_follower()}
								</span>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
