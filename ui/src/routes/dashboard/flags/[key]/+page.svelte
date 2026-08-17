<script lang="ts">
	import type { Pathname } from '$app/types';
	import { resolve } from '$app/paths';
	import { goto, invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import { hasPermission } from '$lib/permissions';
	import { localizeHref } from '$lib/paraglide/runtime';
	import { m } from '$lib/paraglide/messages.js';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// data.environments/selectedEnvironmentId cascade down from
	// dashboard/+layout.server.ts -- the same key can now exist
	// independently per environment, so the detail view needs to say which
	// one it's showing.
	let selectedEnvironmentName = $derived(
		data.environments.find((e) => e.id === data.selectedEnvironmentId)?.name
	);

	const canUpdate = $derived(hasPermission(data, 'flags:update'));
	const canDelete = $derived(hasPermission(data, 'flags:delete'));

	let isSaving = $state(false);
	let saveError = $state('');

	let deleteModal: { show: () => void } | undefined = $state();
	let isDeleting = $state(false);
	let deleteError = $state('');

	function flagUrl() {
		return `/bff/flags/${encodeURIComponent(data.flag.key)}?environmentId=${encodeURIComponent(data.flag.environmentId)}`;
	}

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		const formData = new FormData(event.currentTarget as HTMLFormElement);

		isSaving = true;
		saveError = '';

		const result = await apiRequest(flagUrl(), {
			method: 'PUT',
			body: JSON.stringify({
				enabled: formData.get('enabled') === 'on',
				value: (formData.get('value') ?? '').toString()
			})
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

		const result = await apiRequest(flagUrl(), { method: 'DELETE' });

		isDeleting = false;
		if (result.error) {
			deleteError = resolveErrorMessage(result.code);
			return;
		}
		await goto(resolve(localizeHref('/dashboard') as Pathname));
	}
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
					>{m.flag_detail_version()}</span
				>
				<strong class="text-ink">{data.flag.version}</strong>
			</div>
		</div>

		<form onsubmit={handleSave} class="mt-4 grid gap-4">
			<label class="flex items-center gap-1.5 text-sm text-ink">
				<input type="checkbox" name="enabled" checked={data.flag.enabled} disabled={!canUpdate} />
				{m.flag_edit_enabled_label()}
			</label>
			<label class="grid gap-1.5 text-sm text-ink">
				<span class="font-medium">{m.flag_edit_value_label()}</span>
				<input
					name="value"
					value={data.flag.value}
					disabled={!canUpdate}
					class="w-full rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none disabled:opacity-60"
				/>
			</label>

			{#if saveError}
				<p class="flex items-center gap-2 text-sm text-error">
					<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
					{saveError}
				</p>
			{/if}

			{#if canUpdate}
				<button
					type="submit"
					disabled={isSaving}
					class="cursor-pointer justify-self-start rounded-lg border border-line-1 px-4 py-2 text-sm font-medium text-ink hover:bg-line-3 disabled:cursor-wait disabled:opacity-70"
				>
					{m.flag_edit_submit()}
				</button>
			{/if}
		</form>

		{#if canDelete}
			<div class="mt-6 border-t border-line-4 pt-5">
				{#if deleteError}
					<p class="mb-3 flex items-center gap-2 text-sm text-error">
						<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
						{deleteError}
					</p>
				{/if}
				<button
					type="button"
					disabled={isDeleting}
					class="cursor-pointer text-sm font-medium text-error hover:underline disabled:cursor-wait disabled:opacity-70"
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
	title={m.flag_delete_confirm_title()}
	description={m.flag_delete_confirm_description({ key: data.flag.key })}
	confirmLabel={m.flag_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>
