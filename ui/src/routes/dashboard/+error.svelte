<script lang="ts">
	import type { Pathname } from '$app/types';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { localizeHref } from '$lib/paraglide/runtime';
	import { m } from '$lib/paraglide/messages.js';
</script>

<svelte:head>
	<title>{m.error_title()} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-2xl rounded-xl border border-line-1 bg-surface p-8 text-center">
		<span class="mx-auto mb-4 icon-[lucide--circle-alert] size-10 text-error" aria-hidden="true"
		></span>
		<p class="mb-1 text-xs font-semibold tracking-widest text-nav-active uppercase">
			{m.error_status_label({ status: page.status })}
		</p>
		<h1 class="text-xl font-semibold text-ink">{m.error_title()}</h1>
		<p class="mt-2 text-ink-muted">{page.error?.message}</p>

		<a
			href={resolve(localizeHref('/dashboard') as Pathname)}
			class="mt-6 inline-block rounded-lg bg-gold px-6 py-3 font-semibold text-navy no-underline hover:opacity-90"
		>
			{m.error_back_dashboard()}
		</a>
	</div>
</div>
