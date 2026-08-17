<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import { toast } from '$lib/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import type { ApplicationCredentialSummary } from '$lib/server/applicationCredentials';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const scopeOptions = [
		{ value: 'flags:read', label: m.application_credentials_scope_flags_read },
		{ value: 'flags:write', label: m.application_credentials_scope_flags_write },
		{ value: 'flags:update', label: m.application_credentials_scope_flags_update },
		{ value: 'flags:delete', label: m.application_credentials_scope_flags_delete }
	];

	// This whole page (list, create, edit) is scoped to the environment
	// currently selected in the header (Header.svelte/EnvironmentSwitcher.svelte)
	// -- a credential is only ever scoped to one environment (AC-7.3), and
	// the header selector already establishes "which environment am I
	// working in", so neither creating nor editing a credential asks again.
	// data.environments falls through from the dashboard layout (see
	// +page.server.ts) -- session-scoped, which is exactly the set the
	// header switcher itself offers.
	const selectedEnvironmentName = $derived(
		data.environments.find((e) => e.id === data.selectedEnvironmentId)?.name
	);

	let isCreating = $state(false);
	let createError = $state('');

	let updatingId: string | null = $state(null);
	let updateErrors: Record<string, string> = $state({});

	let deleteModal: { show: () => void } | undefined = $state();
	let pendingDeleteId: string | undefined;
	let pendingDeleteName = $state('');

	let rotateModal: { show: () => void } | undefined = $state();
	let pendingRotateId: string | undefined;
	let pendingRotateName = $state('');
	let rotatingId: string | null = $state(null);

	// Shown exactly once right after a create or rotate response includes a
	// plaintext clientSecret -- there is no other path in this app to
	// retrieve it again afterwards, since list/get responses never include
	// it (mirrors bff/application-credentials' create/rotate-only inclusion).
	let revealedSecret: { clientId: string; clientSecret: string } | null = $state(null);
	let copied = $state(false);

	function dismissRevealedSecret() {
		revealedSecret = null;
		copied = false;
	}

	async function copySecret() {
		if (!revealedSecret) return;
		await navigator.clipboard.writeText(revealedSecret.clientSecret);
		copied = true;
	}

	function requestDelete(credential: ApplicationCredentialSummary) {
		pendingDeleteId = credential.id;
		pendingDeleteName = credential.name;
		deleteModal?.show();
	}

	async function confirmDelete() {
		const id = pendingDeleteId;
		pendingDeleteId = undefined;
		if (!id) return;

		const result = await apiRequest(`/bff/application-credentials/${encodeURIComponent(id)}`, {
			method: 'DELETE'
		});
		if (result.error) {
			toast.show(resolveErrorMessage(result.code));
			return;
		}
		await invalidateAll();
	}

	function requestRotate(credential: ApplicationCredentialSummary) {
		pendingRotateId = credential.id;
		pendingRotateName = credential.name;
		rotateModal?.show();
	}

	async function confirmRotate() {
		const id = pendingRotateId;
		pendingRotateId = undefined;
		if (!id) return;

		rotatingId = id;
		const result = await apiRequest<{ id: string; clientSecret: string }>(
			`/bff/application-credentials/${encodeURIComponent(id)}/rotate`,
			{ method: 'POST' }
		);
		rotatingId = null;
		if (!result.data) {
			toast.show(resolveErrorMessage(result.code));
			return;
		}
		revealedSecret = { clientId: result.data.id, clientSecret: result.data.clientSecret };
		await invalidateAll();
	}

	async function handleUpdate(event: SubmitEvent, credential: ApplicationCredentialSummary) {
		event.preventDefault();
		const formData = new FormData(event.currentTarget as HTMLFormElement);

		updatingId = credential.id;
		updateErrors[credential.id] = '';

		const result = await apiRequest(
			`/bff/application-credentials/${encodeURIComponent(credential.id)}`,
			{
				method: 'PUT',
				body: JSON.stringify({
					name: (formData.get('name') ?? '').toString(),
					// Not user-editable here -- see the comment on
					// selectedEnvironmentName above; a credential never moves
					// environments from this page, so this is always the value
					// it already had.
					environmentId: credential.environmentId,
					scopes: formData.getAll('scopes').map(String),
					active: formData.get('active') === 'on'
				})
			}
		);

		// Only clear updatingId if it's still this row's -- see the identical
		// comment in dashboard/groups/+page.svelte's handleUpdate.
		if (updatingId === credential.id) {
			updatingId = null;
		}
		if (result.error) {
			updateErrors[credential.id] = resolveErrorMessage(result.code);
			return;
		}
		await invalidateAll();
	}

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		if (!data.selectedEnvironmentId) return;
		const formEl = event.currentTarget as HTMLFormElement;
		const formData = new FormData(formEl);

		isCreating = true;
		createError = '';

		const result = await apiRequest<{ id: string; clientSecret: string }>(
			'/bff/application-credentials',
			{
				method: 'POST',
				body: JSON.stringify({
					name: (formData.get('name') ?? '').toString(),
					environmentId: data.selectedEnvironmentId,
					scopes: formData.getAll('scopes').map(String)
				})
			}
		);

		isCreating = false;
		if (!result.data) {
			createError = resolveErrorMessage(result.code);
			return;
		}
		formEl.reset();
		revealedSecret = { clientId: result.data.id, clientSecret: result.data.clientSecret };
		await invalidateAll();
	}
</script>

<svelte:head>
	<title>{m.application_credentials_page_title()} • Aerendil</title>
</svelte:head>

<div class="grid gap-6 p-7">
	<div>
		<h1 class="text-xl font-semibold text-ink">{m.application_credentials_page_title()}</h1>
		<p class="mt-1 text-ink-muted">{m.application_credentials_page_subtitle()}</p>
		{#if selectedEnvironmentName}
			<p class="mt-1 text-sm text-ink-muted">
				{m.flag_detail_environment({ name: selectedEnvironmentName })}
			</p>
		{/if}
	</div>

	{#if revealedSecret}
		<div class="grid gap-3 rounded-xl border border-gold bg-surface p-6">
			<div class="flex items-center gap-2">
				<span class="icon-[lucide--key] size-5 text-gold" aria-hidden="true"></span>
				<h2 class="text-base font-semibold text-ink">
					{m.application_credentials_secret_reveal_title()}
				</h2>
			</div>
			<p class="flex items-center gap-2 text-sm text-error">
				<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
				{m.application_credentials_secret_reveal_warning()}
			</p>
			<div class="grid gap-1.5">
				<span class="text-sm font-medium text-ink"
					>{m.application_credentials_secret_client_id_label()}</span
				>
				<code
					class="overflow-x-auto rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink"
					>{revealedSecret.clientId}</code
				>
			</div>
			<div class="grid gap-1.5">
				<span class="text-sm font-medium text-ink"
					>{m.application_credentials_secret_client_secret_label()}</span
				>
				<div class="flex items-center gap-2">
					<code
						class="flex-1 overflow-x-auto rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink"
						>{revealedSecret.clientSecret}</code
					>
					<button
						type="button"
						class="cursor-pointer rounded-lg border border-line-1 px-4 py-2 text-sm font-medium text-ink hover:bg-line-3"
						onclick={copySecret}
					>
						{copied
							? m.application_credentials_secret_copied()
							: m.application_credentials_secret_copy_button()}
					</button>
				</div>
			</div>
			<button
				type="button"
				class="cursor-pointer justify-self-start rounded-lg bg-gold px-5 py-2.5 font-semibold text-navy hover:opacity-90"
				onclick={dismissRevealedSecret}
			>
				{m.application_credentials_secret_dismiss_button()}
			</button>
		</div>
	{/if}

	<div class="rounded-xl border border-line-1 bg-surface">
		{#if data.applicationCredentials.length === 0}
			<p class="p-6 text-sm text-ink-muted">{m.application_credentials_empty()}</p>
		{:else}
			{#each data.applicationCredentials as credential, i (credential.id)}
				<div class="p-5 {i > 0 ? 'border-t border-line-4' : ''}">
					<div class="flex items-center justify-between gap-3">
						<div class="flex items-center gap-2">
							<strong class="text-ink">{credential.name}</strong>
							<span class="text-sm text-ink-muted">{credential.id}</span>
						</div>
						<div class="flex items-center gap-4">
							<button
								type="button"
								disabled={rotatingId === credential.id}
								class="cursor-pointer text-sm font-medium text-ink hover:underline disabled:cursor-wait disabled:opacity-70"
								onclick={() => requestRotate(credential)}
							>
								{m.application_credentials_rotate_button()}
							</button>
							<button
								type="button"
								class="cursor-pointer text-sm font-medium text-error hover:underline"
								onclick={() => requestDelete(credential)}
							>
								{m.application_credentials_delete_button()}
							</button>
						</div>
					</div>

					<form onsubmit={(event) => handleUpdate(event, credential)} class="mt-3 grid gap-3">
						<input
							name="name"
							value={credential.name}
							class="w-full rounded-lg border border-line-1 bg-page px-4 py-2 text-sm text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
						/>

						<div class="flex flex-wrap gap-3">
							{#each scopeOptions as scope (scope.value)}
								<label class="flex items-center gap-1.5 text-sm text-ink">
									<input
										type="checkbox"
										name="scopes"
										value={scope.value}
										checked={credential.scopes.includes(scope.value)}
									/>
									{scope.label()}
								</label>
							{/each}
						</div>

						<label class="flex items-center gap-1.5 text-sm text-ink">
							<input type="checkbox" name="active" checked={credential.active} />
							{m.application_credentials_edit_active_label()}
						</label>

						{#if updateErrors[credential.id]}
							<p class="flex items-center gap-2 text-sm text-error">
								<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
								{updateErrors[credential.id]}
							</p>
						{/if}

						<button
							type="submit"
							disabled={updatingId === credential.id}
							class="cursor-pointer justify-self-start rounded-lg border border-line-1 px-4 py-2 text-sm font-medium text-ink hover:bg-line-3 disabled:cursor-wait disabled:opacity-70"
						>
							{m.application_credentials_edit_submit()}
						</button>
					</form>
				</div>
			{/each}
		{/if}
	</div>

	<div class="rounded-xl border border-line-1 bg-surface p-6">
		<h2 class="mb-4 text-base font-semibold text-ink">
			{m.application_credentials_create_button()}
		</h2>
		<form onsubmit={handleCreate} class="grid gap-4">
			<label class="grid gap-1.5 text-sm font-medium text-ink">
				<span>{m.application_credentials_create_name_label()}</span>
				<input
					name="name"
					type="text"
					required
					class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
				/>
			</label>

			<div class="grid gap-1.5 text-sm font-medium text-ink">
				<span>{m.application_credentials_create_environment_label()}</span>
				{#if selectedEnvironmentName}
					<p class="rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink">
						{selectedEnvironmentName}
					</p>
				{:else}
					<p class="flex items-center gap-2 text-sm text-error">
						<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
						{m.application_credentials_create_no_environment()}
					</p>
				{/if}
			</div>

			<fieldset class="grid gap-1.5">
				<legend class="text-sm font-medium text-ink"
					>{m.application_credentials_create_scopes_label()}</legend
				>
				<div class="flex flex-wrap gap-3">
					{#each scopeOptions as scope (scope.value)}
						<label class="flex items-center gap-1.5 text-sm text-ink">
							<input type="checkbox" name="scopes" value={scope.value} />
							{scope.label()}
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
				disabled={isCreating || !data.selectedEnvironmentId}
				class="cursor-pointer justify-self-start rounded-lg bg-gold px-5 py-2.5 font-semibold text-navy hover:opacity-90 disabled:cursor-wait disabled:opacity-70"
			>
				{m.application_credentials_create_submit()}
			</button>
		</form>
	</div>
</div>

<ConfirmModal
	bind:this={deleteModal}
	title={m.application_credentials_delete_confirm_title()}
	description={m.application_credentials_delete_confirm_description({ name: pendingDeleteName })}
	confirmLabel={m.application_credentials_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>

<ConfirmModal
	bind:this={rotateModal}
	title={m.application_credentials_rotate_confirm_title()}
	description={m.application_credentials_rotate_confirm_description({ name: pendingRotateName })}
	confirmLabel={m.application_credentials_rotate_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmRotate}
/>
