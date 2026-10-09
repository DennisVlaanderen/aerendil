<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { navigating, page } from '$app/state';
	import DateTimePicker from '#lib/components/DateTimePicker.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatTimestamp } from '#lib/formatDate.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let expandedIds = new SvelteSet<number>();

	// Filters and paging navigate to this same path.
	let loading = $derived(navigating.to?.url.pathname === page.url.pathname);

	// Writable deriveds: reset on navigation, bound so the pickers keep from <= to.
	let from = $derived(data.filter.from);
	let to = $derived(data.filter.to);

	// Swaps only the cursor; none means newest.
	function cursorHref(cursor: string | undefined): string {
		const params = [...page.url.searchParams].filter(([key]) => key !== 'cursor');
		if (cursor) params.push(['cursor', cursor]);
		const query = new URLSearchParams(params).toString();
		return query ? `?${query}` : page.url.pathname;
	}

	// Assumes the page size hasn't changed while paging.
	let pageNumber = $derived(Math.ceil(data.page.end / data.page.limit));
	let pageCount = $derived(Math.ceil(data.page.total / data.page.limit));
	let pagerLinks = $derived([
		{ label: m.audit_log_newest(), cursor: data.filter.cursor ? '' : undefined },
		{ label: m.audit_log_previous(), cursor: data.page.prevCursor },
		{ label: m.audit_log_next(), cursor: data.page.nextCursor }
	]);

	const pagerClass =
		'rounded-lg border border-border bg-surface px-4 py-2 font-medium text-foreground hover:bg-surface-muted';
	const pagerDisabledClass = 'pointer-events-none opacity-50';

	function toggleExpanded(id: number) {
		if (expandedIds.has(id)) {
			expandedIds.delete(id);
		} else {
			expandedIds.add(id);
		}
	}

	// before/after are nested JSON, or a string if not JSON.
	function prettyPrint(value: unknown): string {
		return typeof value === 'string' ? value : JSON.stringify(value, null, 2);
	}
</script>

<svelte:head>
	<title>{m.audit_log_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-foreground">{m.audit_log_page_title()}</h1>
		<p class="mt-1 text-muted-foreground">{m.audit_log_page_subtitle()}</p>
	</div>

	<form
		method="GET"
		class="flex flex-wrap items-end gap-3 rounded-xl border border-border bg-surface p-5"
	>
		<label class="grid gap-1.5 text-sm text-foreground">
			<span class="font-medium">{m.audit_log_filter_target_type()}</span>
			<select
				name="targetType"
				value={data.filter.targetType}
				class="rounded-lg border border-border bg-background py-2 pr-10 pl-4 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
			>
				<option value="">—</option>
				<option value="flag">flag</option>
				<option value="user">user</option>
				<option value="group">group</option>
			</select>
		</label>
		<label class="grid gap-1.5 text-sm text-foreground">
			<span class="font-medium">{m.audit_log_filter_target_id()}</span>
			<input
				name="targetId"
				value={data.filter.targetId}
				class="rounded-lg border border-border bg-background px-4 py-2 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
			/>
		</label>
		<label class="grid gap-1.5 text-sm text-foreground">
			<span class="font-medium">{m.audit_log_filter_actor_id()}</span>
			<input
				name="actorId"
				value={data.filter.actorId}
				class="rounded-lg border border-border bg-background px-4 py-2 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
			/>
		</label>
		<DateTimePicker name="from" label={m.audit_log_filter_from()} bind:value={from} max={to} />
		<DateTimePicker name="to" label={m.audit_log_filter_to()} bind:value={to} min={from} />
		<label class="grid gap-1.5 text-sm text-foreground">
			<span class="font-medium">{m.audit_log_per_page()}</span>
			<select
				name="limit"
				value={String(data.filter.limit)}
				class="rounded-lg border border-border bg-background py-2 pr-10 pl-4 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
			>
				{#each data.pageSizes as size (size)}
					<option value={String(size)}>{size}</option>
				{/each}
			</select>
		</label>
		<button
			type="submit"
			disabled={loading}
			class="flex cursor-pointer items-center gap-2 rounded-lg bg-primary px-5 py-2.25 text-sm font-semibold text-primary-foreground hover:bg-primary-hover disabled:cursor-wait disabled:opacity-70"
		>
			{#if loading}
				<span class="icon-[lucide--loader-circle] size-4 animate-spin" aria-hidden="true"></span>
				{m.audit_log_filter_submitting()}
			{:else}
				{m.audit_log_filter_submit()}
			{/if}
		</button>
	</form>

	<div
		aria-busy={loading}
		class="overflow-x-auto rounded-xl border border-border bg-surface transition-opacity {loading
			? 'pointer-events-none opacity-50'
			: ''}"
	>
		{#if data.entries.length === 0}
			<p class="p-6 text-sm text-muted-foreground">{m.audit_log_empty()}</p>
		{:else}
			<table class="w-full text-left text-sm">
				<thead>
					<tr
						class="border-b border-border text-xs font-semibold tracking-wide text-muted-foreground uppercase"
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
						<tr class="{i > 0 ? 'border-t border-border' : ''} hover:bg-surface-muted">
							<td class="px-3 py-3">
								<button
									type="button"
									class="flex size-6 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-surface-muted"
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
							<td class="px-3 py-3 whitespace-nowrap text-foreground"
								>{formatTimestamp(entry.timestamp)}</td
							>
							<td class="px-3 py-3 text-foreground">
								{entry.actorId}
								{#if entry.actorType === 'applicationCredential'}
									<span
										class="ml-1.5 rounded-full bg-surface-muted px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-muted-foreground uppercase"
									>
										{m.audit_log_actor_type_application()}
									</span>
								{/if}
							</td>
							<td class="px-3 py-3 font-mono text-xs text-foreground">{entry.action}</td>
							<td class="px-3 py-3 text-muted-foreground">
								{entry.targetType}{entry.targetId ? `: ${entry.targetId}` : ''}
							</td>
							<td class="px-3 py-3">
								<span
									class="rounded-full px-2 py-0.5 text-xs font-semibold tracking-wide uppercase {entry.success
										? 'bg-success/10 text-success'
										: 'bg-danger/10 text-danger'}"
								>
									{entry.statusCode}
								</span>
							</td>
						</tr>
						{#if expanded}
							<tr class="border-t border-border">
								<td colspan="6" class="p-5">
									<div class="grid gap-4 sm:grid-cols-2">
										{#if entry.error}
											<div
												class="col-span-full grid gap-1.5 rounded-lg border border-border bg-background p-4"
											>
												<span
													class="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
												>
													{m.audit_log_detail_error()}
												</span>
												<strong class="text-danger">{entry.error}</strong>
											</div>
										{/if}
										{#if entry.before}
											<div class="grid gap-1.5 rounded-lg border border-border bg-background p-4">
												<span
													class="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
												>
													{m.audit_log_detail_before()}
												</span>
												<pre class="overflow-x-auto text-xs text-foreground">{prettyPrint(
														entry.before
													)}</pre>
											</div>
										{/if}
										{#if entry.after}
											<div class="grid gap-1.5 rounded-lg border border-border bg-background p-4">
												<span
													class="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
												>
													{m.audit_log_detail_after()}
												</span>
												<pre class="overflow-x-auto text-xs text-foreground">{prettyPrint(
														entry.after
													)}</pre>
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

	{#if data.page.total > 0}
		<nav
			class="flex flex-wrap items-center justify-between gap-3 text-sm transition-opacity {loading
				? 'pointer-events-none opacity-50'
				: ''}"
		>
			{#if data.page.start > 0}
				<span class="text-muted-foreground">
					{m.audit_log_range({
						start: data.page.start,
						end: data.page.end,
						total: data.page.total
					})}
					·
					{m.audit_log_page_of({ page: pageNumber, pages: pageCount })}
				</span>
			{/if}
			<div class="flex gap-2">
				{#each pagerLinks as link (link.label)}
					{#if link.cursor !== undefined}
						<a href={cursorHref(link.cursor)} class={pagerClass}>{link.label}</a>
					{:else}
						<span aria-disabled="true" class="{pagerClass} {pagerDisabledClass}">{link.label}</span>
					{/if}
				{/each}
			</div>
		</nav>
	{/if}
</div>
