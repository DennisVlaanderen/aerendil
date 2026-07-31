<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { toast } from '$lib/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import type { GroupSummary } from '$lib/server/groups';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let isCreating = $state(false);
	let createError = $state('');

	let updatingId: string | null = $state(null);
	let updateErrors: Record<string, string> = $state({});

	let deleteModal: { show: () => void } | undefined = $state();
	let pendingDeleteId: string | undefined;
	let pendingDeleteGroupName = $state('');

	function requestDelete(group: GroupSummary) {
		pendingDeleteId = group.id;
		pendingDeleteGroupName = group.name;
		deleteModal?.show();
	}

	async function confirmDelete() {
		const id = pendingDeleteId;
		pendingDeleteId = undefined;
		if (!id) return;

		const result = await apiRequest(`/bff/groups/${encodeURIComponent(id)}`, {
			method: 'DELETE'
		});
		if (result.error) {
			toast.show(result.error);
			return;
		}
		await invalidateAll();
	}

	async function handleUpdate(event: SubmitEvent, group: GroupSummary) {
		event.preventDefault();
		const data = new FormData(event.currentTarget as HTMLFormElement);

		updatingId = group.id;
		updateErrors[group.id] = '';

		const result = await apiRequest(`/bff/groups/${encodeURIComponent(group.id)}`, {
			method: 'PUT',
			body: JSON.stringify({
				name: (data.get('name') ?? '').toString(),
				permissions: data.getAll('permissions').map(String),
				environmentIds: data.getAll('environmentIds').map(String)
			})
		});

		// Only clear updatingId if it's still this row's -- see the identical
		// comment in dashboard/users/+page.svelte's handleUpdate.
		if (updatingId === group.id) {
			updatingId = null;
		}
		if (result.error) {
			updateErrors[group.id] = result.error;
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

		const result = await apiRequest('/bff/groups', {
			method: 'POST',
			body: JSON.stringify({
				name: (data.get('name') ?? '').toString(),
				permissions: data.getAll('permissions').map(String),
				environmentIds: data.getAll('environmentIds').map(String)
			})
		});

		isCreating = false;
		if (result.error) {
			createError = result.error;
			return;
		}
		formEl.reset();
		await invalidateAll();
	}
</script>

<svelte:head>
	<title>{m.groups_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-ink">{m.groups_page_title()}</h1>
		<p class="mt-1 text-ink-muted">{m.groups_page_subtitle()}</p>
	</div>

	<div class="rounded-xl border border-line-1 bg-surface">
		{#if data.groups.length === 0}
			<p class="p-6 text-sm text-ink-muted">{m.groups_empty()}</p>
		{:else}
			{#each data.groups as group, i (group.id)}
				<div class="p-5 {i > 0 ? 'border-t border-line-4' : ''}">
					<div class="flex items-center justify-between gap-3">
						<div class="flex items-center gap-2">
							<strong class="text-ink">{group.name}</strong>
							{#if group.system}
								<span
									class="rounded-full bg-control px-2 py-0.5 text-xs font-semibold tracking-wide text-ink-muted uppercase"
								>
									{m.groups_system_badge()}
								</span>
							{/if}
						</div>
						{#if !group.system}
							<button
								type="button"
								class="cursor-pointer text-sm font-medium text-error hover:underline"
								onclick={() => requestDelete(group)}
							>
								{m.groups_delete_button()}
							</button>
						{/if}
					</div>

					{#if group.system}
						<p class="mt-1 text-sm text-ink-muted">{m.groups_admin_protected_hint()}</p>
					{:else}
						<form onsubmit={(event) => handleUpdate(event, group)} class="mt-3 grid gap-3">
							<input
								name="name"
								value={group.name}
								class="w-full rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
							/>
							<div class="flex flex-wrap gap-3">
								{#each data.allPermissions as perm (perm)}
									<label class="flex items-center gap-1.5 text-sm text-ink">
										<input
											type="checkbox"
											name="permissions"
											value={perm}
											checked={group.permissions.includes(perm)}
										/>
										{perm}
									</label>
								{/each}
							</div>

							<fieldset class="grid gap-1.5">
								<legend class="text-sm font-medium text-ink"
									>{m.groups_create_environments_label()}</legend
								>
								<div class="flex flex-wrap gap-3">
									{#each data.environments as environment (environment.id)}
										<label class="flex items-center gap-1.5 text-sm text-ink">
											<input
												type="checkbox"
												name="environmentIds"
												value={environment.id}
												checked={group.environmentIds.includes(environment.id)}
											/>
											{environment.name}
										</label>
									{/each}
								</div>
							</fieldset>

							{#if updateErrors[group.id]}
								<p class="flex items-center gap-2 text-sm text-error">
									<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"
									></span>
									{updateErrors[group.id]}
								</p>
							{/if}

							<button
								type="submit"
								disabled={updatingId === group.id}
								class="cursor-pointer justify-self-start rounded-lg border border-line-1 px-4 py-2 text-sm font-medium text-ink hover:bg-line-3 disabled:cursor-wait disabled:opacity-70"
							>
								{m.groups_edit_submit()}
							</button>
						</form>
					{/if}
				</div>
			{/each}
		{/if}
	</div>

	<div class="rounded-xl border border-line-1 bg-surface p-6">
		<h2 class="mb-4 text-base font-semibold text-ink">{m.groups_create_button()}</h2>
		<form onsubmit={handleCreate} class="grid gap-4">
			<label class="grid gap-1.5 text-sm font-medium text-ink">
				<span>{m.groups_create_name_label()}</span>
				<input
					name="name"
					type="text"
					required
					class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
				/>
			</label>

			<fieldset class="grid gap-1.5">
				<legend class="text-sm font-medium text-ink">{m.groups_create_permissions_label()}</legend>
				<div class="flex flex-wrap gap-3">
					{#each data.allPermissions as perm (perm)}
						<label class="flex items-center gap-1.5 text-sm text-ink">
							<input type="checkbox" name="permissions" value={perm} />
							{perm}
						</label>
					{/each}
				</div>
			</fieldset>

			<fieldset class="grid gap-1.5">
				<legend class="text-sm font-medium text-ink">{m.groups_create_environments_label()}</legend>
				<div class="flex flex-wrap gap-3">
					{#each data.environments as environment (environment.id)}
						<label class="flex items-center gap-1.5 text-sm text-ink">
							<input type="checkbox" name="environmentIds" value={environment.id} />
							{environment.name}
						</label>
					{/each}
				</div>
			</fieldset>

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
				{m.groups_create_submit()}
			</button>
		</form>
	</div>
</div>

<ConfirmModal
	bind:this={deleteModal}
	title={m.groups_delete_confirm_title()}
	description={m.groups_delete_confirm_description({ name: pendingDeleteGroupName })}
	confirmLabel={m.groups_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>
