<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { m } from '$lib/paraglide/messages.js';
	import { formatTimestamp } from '$lib/formatDate';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let expandedIds = new SvelteSet<number>();

	function toggleExpanded(id: number) {
		if (expandedIds.has(id)) {
			expandedIds.delete(id);
		} else {
			expandedIds.add(id);
		}
	}

	// before/after are JSON-encoded strings (see AuditEntry in
	// lib/server/auditLog.ts) -- pretty-print when parseable, otherwise fall
	// back to showing the raw string rather than hiding the data.
	function prettyPrint(value: string | undefined): string | null {
		if (!value) return null;
		try {
			return JSON.stringify(JSON.parse(value), null, 2);
		} catch {
			return value;
		}
	}
</script>

<svelte:head>
	<title>{m.audit_log_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-ink">{m.audit_log_page_title()}</h1>
		<p class="mt-1 text-ink-muted">{m.audit_log_page_subtitle()}</p>
	</div>

	<form
		method="GET"
		class="flex flex-wrap items-end gap-3 rounded-xl border border-line-1 bg-surface p-5"
	>
		<label class="grid gap-1.5 text-sm text-ink">
			<span class="font-medium">{m.audit_log_filter_target_type()}</span>
			<select
				name="targetType"
				value={data.filter.targetType}
				class="rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
			>
				<option value="">—</option>
				<option value="flag">flag</option>
				<option value="user">user</option>
				<option value="group">group</option>
			</select>
		</label>
		<label class="grid gap-1.5 text-sm text-ink">
			<span class="font-medium">{m.audit_log_filter_target_id()}</span>
			<input
				name="targetId"
				value={data.filter.targetId}
				class="rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
			/>
		</label>
		<label class="grid gap-1.5 text-sm text-ink">
			<span class="font-medium">{m.audit_log_filter_actor_id()}</span>
			<input
				name="actorId"
				value={data.filter.actorId}
				class="rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
			/>
		</label>
		<button
			type="submit"
			class="cursor-pointer rounded-lg bg-gold px-5 py-2.25 text-sm font-semibold text-navy hover:opacity-90"
		>
			{m.audit_log_filter_submit()}
		</button>
	</form>

	<div class="overflow-x-auto rounded-xl border border-line-1 bg-surface">
		{#if data.entries.length === 0}
			<p class="p-6 text-sm text-ink-muted">{m.audit_log_empty()}</p>
		{:else}
			<table class="w-full text-left text-sm">
				<thead>
					<tr
						class="border-b border-line-4 text-xs font-semibold tracking-wide text-ink-muted uppercase"
					>
						<th class="w-10 px-3 py-3"></th>
						<th class="px-3 py-3">{m.audit_log_table_timestamp()}</th>
						<th class="px-3 py-3">{m.audit_log_table_actor()}</th>
						<th class="px-3 py-3">{m.audit_log_table_action()}</th>
						<th class="px-3 py-3">{m.audit_log_table_target()}</th>
						<th class="px-3 py-3">{m.audit_log_table_status()}</th>
					</tr>
				</thead>
				<tbody>
					{#each data.entries as entry, i (entry.id)}
						{@const expanded = expandedIds.has(entry.id)}
						<tr class="{i > 0 ? 'border-t border-line-4' : ''} hover:bg-line-3">
							<td class="px-3 py-3">
								<button
									type="button"
									class="flex size-6 cursor-pointer items-center justify-center rounded-md text-ink-muted hover:bg-line-3"
									aria-expanded={expanded}
									aria-label={m.audit_log_toggle_details()}
									onclick={() => toggleExpanded(entry.id)}
								>
									<span
										class="icon-[lucide--chevron-right] size-4 transition-transform duration-150 {expanded
											? 'rotate-90'
											: ''}"
										aria-hidden="true"
									></span>
								</button>
							</td>
							<td class="px-3 py-3 whitespace-nowrap text-ink">{formatTimestamp(entry.timestamp)}</td>
							<td class="px-3 py-3 text-ink">{entry.actorId}</td>
							<td class="px-3 py-3 font-mono text-xs text-ink">{entry.action}</td>
							<td class="px-3 py-3 text-ink-muted">
								{entry.targetType}{entry.targetId ? `: ${entry.targetId}` : ''}
							</td>
							<td class="px-3 py-3">
								<span
									class="rounded-full px-2 py-0.5 text-xs font-semibold tracking-wide uppercase {entry.success
										? 'bg-success-bg text-success'
										: 'bg-control text-error'}"
								>
									{entry.statusCode}
								</span>
							</td>
						</tr>
						{#if expanded}
							<tr class="border-t border-line-4">
								<td colspan="6" class="p-5">
									<div class="grid gap-4 sm:grid-cols-2">
										{#if entry.error}
											<div
												class="col-span-full grid gap-1.5 rounded-lg border border-line-1 bg-page p-4"
											>
												<span
													class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
												>
													{m.audit_log_detail_error()}
												</span>
												<strong class="text-error">{entry.error}</strong>
											</div>
										{/if}
										{#if entry.before}
											<div class="grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
												<span
													class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
												>
													{m.audit_log_detail_before()}
												</span>
												<pre class="overflow-x-auto text-xs text-ink">{prettyPrint(entry.before)}</pre>
											</div>
										{/if}
										{#if entry.after}
											<div class="grid gap-1.5 rounded-lg border border-line-1 bg-page p-4">
												<span
													class="text-xs font-semibold tracking-wider text-ink-muted uppercase"
												>
													{m.audit_log_detail_after()}
												</span>
												<pre class="overflow-x-auto text-xs text-ink">{prettyPrint(entry.after)}</pre>
											</div>
										{/if}
									</div>
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</div>
