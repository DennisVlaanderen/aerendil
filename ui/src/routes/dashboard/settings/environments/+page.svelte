<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import { toast } from '$lib/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import type { EnvironmentSummary } from '$lib/server/environments';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let isCreating = $state(false);
	let createError = $state('');

	let updatingId: string | null = $state(null);
	let updateErrors: Record<string, string> = $state({});

	let deleteModal: { show: () => void } | undefined = $state();
	let pendingDeleteId: string | undefined;
	let pendingDeleteEnvironmentName = $state('');

	function requestDelete(environment: EnvironmentSummary) {
		pendingDeleteId = environment.id;
		pendingDeleteEnvironmentName = environment.name;
		deleteModal?.show();
	}

	async function confirmDelete() {
		const id = pendingDeleteId;
		pendingDeleteId = undefined;
		if (!id) return;

		const result = await apiRequest(`/bff/environments/${encodeURIComponent(id)}`, {
			method: 'DELETE'
		});
		if (result.error) {
			toast.show(resolveErrorMessage(result.code));
			return;
		}
		await invalidateAll();
	}

	async function handleUpdate(event: SubmitEvent, environment: EnvironmentSummary) {
		event.preventDefault();
		const data = new FormData(event.currentTarget as HTMLFormElement);

		updatingId = environment.id;
		updateErrors[environment.id] = '';

		const result = await apiRequest(`/bff/environments/${encodeURIComponent(environment.id)}`, {
			method: 'PUT',
			body: JSON.stringify({
				name: (data.get('name') ?? '').toString()
			})
		});

		// Only clear updatingId if it's still this row's -- see the identical
		// comment in dashboard/groups/+page.svelte's handleUpdate.
		if (updatingId === environment.id) {
			updatingId = null;
		}
		if (result.error) {
			updateErrors[environment.id] = resolveErrorMessage(result.code);
			return;
		}
		await invalidateAll();
	}

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		const formEl = event.currentTarget as HTMLFormElement;
		const data = new FormData(formEl);

		isCreating = true;
		createError = '';

		const result = await apiRequest('/bff/environments', {
			method: 'POST',
			body: JSON.stringify({
				name: (data.get('name') ?? '').toString()
			})
		});

		isCreating = false;
		if (result.error) {
			createError = resolveErrorMessage(result.code);
			return;
		}
		formEl.reset();
		await invalidateAll();
	}
</script>

<svelte:head>
	<title>{m.environments_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-ink">{m.environments_page_title()}</h1>
		<p class="mt-1 text-ink-muted">{m.environments_page_subtitle()}</p>
	</div>

	<div class="rounded-xl border border-line-1 bg-surface">
		{#if data.environments.length === 0}
			<p class="p-6 text-sm text-ink-muted">{m.environments_empty()}</p>
		{:else}
			{#each data.environments as environment, i (environment.id)}
				<div class="p-5 {i > 0 ? 'border-t border-line-4' : ''}">
					<div class="flex items-center justify-between gap-3">
						<div class="flex items-center gap-2">
							<strong class="text-ink">{environment.name}</strong>
							<span class="text-sm text-ink-muted"
								>{m.environments_table_order()}: {environment.order}</span
							>
						</div>
						<button
							type="button"
							class="cursor-pointer text-sm font-medium text-error hover:underline"
							onclick={() => requestDelete(environment)}
						>
							{m.environments_delete_button()}
						</button>
					</div>

					<form onsubmit={(event) => handleUpdate(event, environment)} class="mt-3 grid gap-3">
						<input
							name="name"
							value={environment.name}
							class="w-full rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
						/>

						{#if updateErrors[environment.id]}
							<p class="flex items-center gap-2 text-sm text-error">
								<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
								{updateErrors[environment.id]}
							</p>
						{/if}

						<button
							type="submit"
							disabled={updatingId === environment.id}
							class="cursor-pointer justify-self-start rounded-lg border border-line-1 px-4 py-2 text-sm font-medium text-ink hover:bg-line-3 disabled:cursor-wait disabled:opacity-70"
						>
							{m.environments_edit_submit()}
						</button>
					</form>
				</div>
			{/each}
		{/if}
	</div>

	<div class="rounded-xl border border-line-1 bg-surface p-6">
		<h2 class="mb-4 text-base font-semibold text-ink">{m.environments_create_button()}</h2>
		<form onsubmit={handleCreate} class="grid gap-4">
			<label class="grid gap-1.5 text-sm font-medium text-ink">
				<span>{m.environments_create_name_label()}</span>
				<input
					name="name"
					type="text"
					required
					class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
				/>
			</label>

			{#if createError}
				<p class="flex items-center gap-2 text-sm text-error">
					<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
					{createError}
				</p>
			{/if}

			<button
				type="submit"
				disabled={isCreating}
				class="cursor-pointer justify-self-start rounded-lg bg-gold px-5 py-2.5 font-semibold text-navy hover:opacity-90 disabled:cursor-wait disabled:opacity-70"
			>
				{m.environments_create_submit()}
			</button>
		</form>
	</div>
</div>

<ConfirmModal
	bind:this={deleteModal}
	title={m.environments_delete_confirm_title()}
	description={m.environments_delete_confirm_description({ name: pendingDeleteEnvironmentName })}
	confirmLabel={m.environments_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>
