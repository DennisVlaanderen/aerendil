<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Pathname } from '$app/types';
	import { localizeHref } from '$lib/paraglide/runtime';
	import { apiRequest } from '$lib/client/api';
	import { m } from '$lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let isCreating = $state(false);
	let createError = $state('');

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		const formEl = event.currentTarget as HTMLFormElement;
		const formData = new FormData(formEl);
		const key = (formData.get('key') ?? '').toString();
		const environmentIds = formData.getAll('environmentIds').map(String);

		isCreating = true;
		createError = '';

		const result = await apiRequest('/bff/flags', {
			method: 'POST',
			body: JSON.stringify({
				key,
				enabled: formData.get('enabled') === 'on',
				value: (formData.get('value') ?? '').toString(),
				environmentIds
			})
		});

		isCreating = false;
		if (result.error) {
			createError = result.error;
			return;
		}

		await invalidateAll();
		// The flag detail page is scoped to the currently-selected
		// environment (dashboard/flags/[key]/+page.server.ts) -- only jump
		// there if that's actually one of the environments this flag just
		// landed in, otherwise the detail page would 404.
		if (environmentIds.includes(data.selectedEnvironmentId ?? '')) {
			await goto(resolve(localizeHref(`/dashboard/flags/${encodeURIComponent(key)}`) as Pathname));
		} else {
			await goto(resolve(localizeHref('/dashboard') as Pathname));
		}
	}
</script>

<svelte:head>
	<title>{m.flag_create_page_title()} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-2xl rounded-xl border border-line-1 bg-surface p-6">
		<h1 class="mb-1 text-xl font-semibold text-ink">{m.flag_create_page_title()}</h1>
		<p class="mb-5 text-ink-muted">{m.flag_create_page_subtitle()}</p>

		{#if data.environments.length === 0}
			<p class="text-sm text-ink-muted">{m.flag_create_no_environments()}</p>
		{:else}
			<form onsubmit={handleCreate} class="grid gap-4">
				<label class="grid gap-1.5 text-sm font-medium text-ink">
					<span>{m.flag_create_key_label()}</span>
					<input
						name="key"
						type="text"
						required
						class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
					/>
				</label>

				<label class="grid gap-1.5 text-sm font-medium text-ink">
					<span>{m.flag_create_value_label()}</span>
					<input
						name="value"
						type="text"
						class="w-full rounded-lg border border-line-1 bg-page px-4 py-2.5 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
					/>
				</label>

				<label class="flex items-center gap-2 text-sm font-medium text-ink">
					<input type="checkbox" name="enabled" />
					{m.flag_create_enabled_label()}
				</label>

				<fieldset class="grid gap-1.5">
					<legend class="text-sm font-medium text-ink">{m.flag_create_environments_label()}</legend>
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
					{m.flag_create_submit()}
				</button>
			</form>
		{/if}
	</div>
</div>
