<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import { hasPermission } from '$lib/permissions';
	import { localizedResolve } from '$lib/localizedResolve';
	import { m } from '$lib/paraglide/messages.js';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const canUpdate = $derived(hasPermission(data, 'users:update'));

	let isSaving = $state(false);
	let saveError = $state('');

	let deleteModal: { show: () => void } | undefined = $state();
	let isDeleting = $state(false);
	let deleteError = $state('');

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		const formData = new FormData(event.currentTarget as HTMLFormElement);

		isSaving = true;
		saveError = '';

		// groupIds is only included when the checkboxes were actually
		// rendered (data.groups.length > 0, see the template below) -- see
		// the identical comment previously on dashboard/users/+page.svelte's
		// handleUpdate for why omitting it matters.
		const payload: Record<string, unknown> = {
			username: (formData.get('username') ?? '').toString(),
			password: (formData.get('password') ?? '').toString(),
			active: formData.get('active') === 'on'
		};
		if (data.groups.length > 0) {
			payload.groupIds = formData.getAll('groupIds').map(String);
		}

		const result = await apiRequest(`/bff/users/${encodeURIComponent(data.user.id)}`, {
			method: 'PUT',
			body: JSON.stringify(payload)
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

		const result = await apiRequest(`/bff/users/${encodeURIComponent(data.user.id)}`, {
			method: 'DELETE'
		});

		isDeleting = false;
		if (result.error) {
			deleteError = resolveErrorMessage(result.code);
			return;
		}
		await goto(localizedResolve('/dashboard/users'));
	}
</script>

<svelte:head>
	<title>{data.user.username} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-2xl rounded-xl border border-line-1 bg-surface p-6">
		<a
			class="mb-1 inline-block text-xs font-semibold tracking-widest text-nav-active uppercase no-underline"
			href={localizedResolve('/dashboard/users')}
		>
			{m.nav_users()}
		</a>
		<h1 class="mb-5 flex items-center gap-3 text-xl font-semibold break-words text-ink">
			<span class="icon-[lucide--user] size-6 shrink-0 text-gold" aria-hidden="true"></span>
			{data.user.username}
		</h1>

		<form onsubmit={handleSave} class="grid gap-4">
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="grid gap-1.5 text-sm text-ink">
					<span class="font-medium">{m.users_table_username()}</span>
					<input
						name="username"
						value={data.user.username}
						disabled={!canUpdate}
						class="w-full rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none disabled:opacity-60"
					/>
				</label>
				<label class="grid gap-1.5 text-sm text-ink">
					<span class="font-medium">{m.users_edit_password_label()}</span>
					<input
						name="password"
						type="password"
						placeholder={m.users_edit_password_placeholder()}
						disabled={!canUpdate}
						class="w-full rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none disabled:opacity-60"
					/>
				</label>
			</div>

			{#if data.groups.length > 0}
				<div class="flex flex-wrap gap-3">
					{#each data.groups as group (group.id)}
						<label class="flex items-center gap-1.5 text-sm text-ink">
							<input
								type="checkbox"
								name="groupIds"
								value={group.id}
								checked={data.user.groupIds.includes(group.id)}
								disabled={!canUpdate}
							/>
							{group.name}
						</label>
					{/each}
				</div>
			{/if}

			<label class="flex items-center gap-1.5 text-sm text-ink">
				<input type="checkbox" name="active" checked={data.user.active} disabled={!canUpdate} />
				{m.users_edit_active_label()}
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
					{m.users_edit_submit()}
				</button>
			{/if}
		</form>

		{#if data.isAdmin}
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
					{m.users_delete_button()}
				</button>
			</div>
		{/if}
	</div>
</div>

<ConfirmModal
	bind:this={deleteModal}
	title={m.users_delete_confirm_title()}
	description={m.users_delete_confirm_description({ username: data.user.username })}
	confirmLabel={m.users_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>
