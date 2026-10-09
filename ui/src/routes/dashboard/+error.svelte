<script lang="ts">
	import { page } from '$app/state';
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { m } from '#lib/paraglide/messages.js';
</script>

<svelte:head>
	<title>{m.error_title()} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-2xl rounded-xl border border-border bg-surface p-8 text-center">
		<span class="mx-auto mb-4 icon-[lucide--circle-alert] size-10 text-danger" aria-hidden="true"
		></span>
		<p class="mb-1 text-xs font-semibold tracking-widest text-primary uppercase">
			{m.error_status_label({ status: page.status })}
		</p>
		<h1 class="text-xl font-semibold text-foreground">{m.error_title()}</h1>
		<p class="mt-2 text-muted-foreground">{page.error?.message}</p>

		<a
			href={localizedResolve('/dashboard')}
			class="mt-6 inline-block rounded-lg bg-primary px-6 py-3 font-semibold text-primary-foreground no-underline hover:bg-primary-hover"
		>
			{m.error_back_dashboard()}
		</a>
	</div>
</div>
