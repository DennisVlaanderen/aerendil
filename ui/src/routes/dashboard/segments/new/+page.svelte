<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { apiRequest } from '#lib/client/api.ts';
	import { resolveErrorMessage } from '#lib/errors.ts';
	import { m } from '#lib/paraglide/messages.js';
	import SegmentForm from '#lib/components/SegmentForm.svelte';
	import type { Segment } from '#lib/flags.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let isCreating = $state(false);
	let createError = $state('');
	const environmentName = $derived(
		data.environments.find((e) => e.id === data.selectedEnvironmentId)?.name ?? ''
	);

	async function create(segment: Partial<Segment>) {
		isCreating = true;
		createError = '';
		const result = await apiRequest('/bff/segments', {
			method: 'POST',
			body: JSON.stringify({ ...segment, environmentId: data.selectedEnvironmentId })
		});
		isCreating = false;
		if (result.error) {
			createError = resolveErrorMessage(result.code);
			return;
		}
		await invalidateAll();
		await goto(localizedResolve(`/dashboard/segments/${encodeURIComponent(segment.key ?? '')}`));
	}
</script>

<svelte:head>
	<title>{m.segment_create_page_title()} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-4xl rounded-xl border border-border bg-surface p-6">
		<h1 class="mb-1 text-xl font-semibold text-foreground">{m.segment_create_page_title()}</h1>
		<p class="mb-5 text-muted-foreground">
			{m.segment_create_page_subtitle({ name: environmentName })}
		</p>
		<SegmentForm
			segment={{}}
			creating
			saving={isCreating}
			error={createError}
			submitLabel={m.segment_create_submit()}
			onsave={create}
		/>
	</div>
</div>
