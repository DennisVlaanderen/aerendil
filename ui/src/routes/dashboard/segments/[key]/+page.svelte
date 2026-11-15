<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { apiRequest } from '#lib/client/api.ts';
	import { resolveErrorMessage } from '#lib/errors.ts';
	import { hasPermission } from '#lib/permissions.ts';
	import { m } from '#lib/paraglide/messages.js';
	import ConfirmModal from '#lib/components/ConfirmModal.svelte';
	import SegmentForm from '#lib/components/SegmentForm.svelte';
	import { segmentUsage, type Segment } from '#lib/flags.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const canUpdate = $derived(hasPermission(data, 'segments:update'));
	const canDelete = $derived(hasPermission(data, 'segments:delete'));
	// The backend refuses deleting a segment a flag still uses (§6.2).
	const usedBy = $derived(segmentUsage(data.flags, data.segment.key));

	let isSaving = $state(false);
	let saveError = $state('');
	let deleteModal: { show: () => void } | undefined = $state();
	let isDeleting = $state(false);
	let deleteError = $state('');

	function segmentUrl() {
		return `/bff/segments/${encodeURIComponent(data.segment.key)}?environmentId=${encodeURIComponent(data.segment.environmentId)}`;
	}

	async function save(segment: Partial<Segment>) {
		isSaving = true;
		saveError = '';
		const result = await apiRequest(segmentUrl(), {
			method: 'PUT',
			body: JSON.stringify(segment)
		});
		isSaving = false;
		if (result.error) {
			saveError = resolveErrorMessage(result.code);
			return;
		}
		await invalidateAll();
	}

	async function confirmDelete() {
		isDeleting = true;
		deleteError = '';
		const result = await apiRequest(segmentUrl(), { method: 'DELETE' });
		isDeleting = false;
		if (result.error) {
			deleteError = resolveErrorMessage(result.code);
			return;
		}
		await goto(localizedResolve('/dashboard/segments'));
	}
</script>

<svelte:head>
	<title>{data.segment.key} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-4xl rounded-xl border border-border bg-surface p-6">
		<p class="mb-1 text-xs font-semibold tracking-widest text-primary uppercase">
			{m.nav_segments()}
		</p>
		<h1 class="mb-1 text-xl font-semibold break-words text-foreground">{data.segment.key}</h1>
		<p class="mb-5 text-sm text-muted-foreground">
			{m.segment_used_by()}:
			{#each usedBy as key, i (key)}
				<a
					class="font-mono text-primary hover:underline"
					href={localizedResolve(`/dashboard/flags/${encodeURIComponent(key)}`)}>{key}</a
				>{i < usedBy.length - 1 ? ', ' : ''}
			{:else}
				{m.segment_unused()}
			{/each}
		</p>

		<SegmentForm
			segment={data.segment}
			disabled={!canUpdate}
			saving={isSaving}
			error={saveError}
			submitLabel={m.flag_edit_submit()}
			onsave={save}
		/>

		{#if canDelete}
			<div class="mt-6 border-t border-border pt-5">
				{#if deleteError}
					<p class="mb-3 flex items-center gap-2 text-sm text-danger">
						<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
						{deleteError}
					</p>
				{/if}
				{#if usedBy.length > 0}
					<p class="mb-3 text-sm text-muted-foreground">{m.segment_delete_blocked()}</p>
				{/if}
				<button
					type="button"
					disabled={isDeleting || usedBy.length > 0}
					class="cursor-pointer text-sm font-medium text-danger hover:underline disabled:cursor-not-allowed disabled:opacity-50"
					onclick={() => deleteModal?.show()}
				>
					{m.flag_delete_button()}
				</button>
			</div>
		{/if}
	</div>
</div>

<ConfirmModal
	bind:this={deleteModal}
	title={m.segment_delete_confirm_title()}
	description={m.flag_delete_confirm_description({ key: data.segment.key })}
	confirmLabel={m.flag_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>
