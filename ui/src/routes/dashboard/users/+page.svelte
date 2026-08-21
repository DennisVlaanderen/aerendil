<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import { toast } from '$lib/toast.svelte';
	import { hasPermission } from '$lib/permissions';
	import { localizedResolve } from '$lib/localizedResolve';
	import { m } from '$lib/paraglide/messages.js';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import type { UserSummary } from '$lib/server/users';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const canCreate = $derived(hasPermission(data, 'users:create'));
	const canUpdate = $derived(hasPermission(data, 'users:update'));

	function groupNames(user: UserSummary): string {
		return data.groups
			.filter((group) => user.groupIds.includes(group.id))
			.map((group) => group.name)
			.join(', ');
	}

	let isCreating = $state(false);
	let createError = $state('');

	let toggleModal: { show: () => void } | undefined = $state();
	let pendingToggleUser: UserSummary | undefined = $state();

	function requestToggle(user: UserSummary) {
		pendingToggleUser = user;
		toggleModal?.show();
	}

	async function confirmToggle() {
		const user = pendingToggleUser;
		pendingToggleUser = undefined;
		if (!user) return;

		// Omitting groupIds leaves membership unchanged -- same convention
		// updateUser documents (lib/server/users.ts).
		const result = await apiRequest(`/bff/users/${encodeURIComponent(user.id)}`, {
			method: 'PUT',
			body: JSON.stringify({ username: user.username, password: '', active: !user.active })
		});
		if (result.error) {
			toast.show(resolveErrorMessage(result.code));
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

		const result = await apiRequest('/bff/users', {
			method: 'POST',
			body: JSON.stringify({
				username: (data.get('username') ?? '').toString(),
				password: (data.get('password') ?? '').toString(),
				groupIds: data.getAll('groupIds').map(String)
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
	<title>{m.users_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-ink">{m.users_page_title()}</h1>
		<p class="mt-1 text-ink-muted">{m.users_page_subtitle()}</p>
	</div>

	<div class="rounded-xl border border-line-1 bg-surface">
		{#if data.users.length === 0}
			<p class="p-6 text-sm text-ink-muted">{m.users_empty()}</p>
		{:else}
			{#each data.users as user, i (user.id)}
				<div
					class="flex items-start justify-between gap-3 p-5 {i > 0 ? 'border-t border-line-4' : ''}"
				>
					<div class="grid min-w-0 gap-1">
						<div class="flex items-center gap-2">
							<strong class="truncate text-ink">{user.username}</strong>
							<span
								class="rounded-full px-2 py-0.5 text-xs font-semibold tracking-wide uppercase {user.active
									? 'bg-success-bg text-success'
									: 'bg-control text-ink-muted'}"
							>
								{user.active ? m.users_status_active() : m.users_status_inactive()}
							</span>
						</div>
						<p class="truncate text-xs text-ink-muted">{groupNames(user) || m.users_no_groups()}</p>
					</div>
					<div class="flex shrink-0 items-center gap-1">
						{#if canUpdate}
							<button
								type="button"
								class="flex size-8 cursor-pointer items-center justify-center rounded-lg text-ink-muted hover:bg-line-3"
								aria-label={user.active ? m.users_disable_button() : m.users_enable_button()}
								onclick={() => requestToggle(user)}
							>
								{#if user.active}
									<span class="icon-[lucide--ban] size-4" aria-hidden="true"></span>
								{:else}
									<span class="icon-[lucide--check-circle] size-4" aria-hidden="true"></span>
								{/if}
							</button>
						{/if}
						<a
							href={localizedResolve(`/dashboard/users/${user.id}`)}
							class="flex size-8 items-center justify-center rounded-lg text-ink-muted no-underline hover:bg-line-3"
							aria-label={m.users_view_details()}
						>
							<span class="icon-[lucide--chevron-right] size-4" aria-hidden="true"></span>
						</a>
					</div>
				</div>
			{/each}
		{/if}
	</div>

	{#if canCreate}
		<div class="rounded-xl border border-line-1 bg-surface p-6">
			<h2 class="mb-4 text-base font-semibold text-ink">{m.users_create_button()}</h2>
			<form onsubmit={handleCreate} class="grid gap-4">
				<label class="grid gap-1.5 text-sm font-medium text-ink">
					<span>{m.users_create_username_label()}</span>
					<input
						name="username"
						type="text"
						required
						class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
					/>
				</label>

				<label class="grid gap-1.5 text-sm font-medium text-ink">
					<span>{m.users_create_password_label()}</span>
					<input
						name="password"
						type="password"
						required
						class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
					/>
				</label>

				{#if data.groups.length > 0}
					<fieldset class="grid gap-1.5">
						<legend class="text-sm font-medium text-ink">{m.users_create_groups_label()}</legend>
						<div class="flex flex-wrap gap-3">
							{#each data.groups as group (group.id)}
								<label class="flex items-center gap-1.5 text-sm text-ink">
									<input type="checkbox" name="groupIds" value={group.id} />
									{group.name}
								</label>
							{/each}
						</div>
					</fieldset>
				{/if}

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
					{m.users_create_submit()}
				</button>
			</form>
		</div>
	{/if}
</div>

<ConfirmModal
	bind:this={toggleModal}
	title={pendingToggleUser?.active
		? m.users_disable_confirm_title()
		: m.users_enable_confirm_title()}
	description={pendingToggleUser?.active
		? m.users_disable_confirm_description({ username: pendingToggleUser?.username ?? '' })
		: m.users_enable_confirm_description({ username: pendingToggleUser?.username ?? '' })}
	confirmLabel={pendingToggleUser?.active ? m.users_disable_button() : m.users_enable_button()}
	cancelLabel={m.modal_cancel()}
	variant={pendingToggleUser?.active ? 'danger' : 'default'}
	onconfirm={confirmToggle}
/>
