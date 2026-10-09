<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { m } from '#lib/paraglide/messages.js';

	const REDIRECT_SECONDS = 20;
	let secondsLeft = $state(REDIRECT_SECONDS);

	function goToLogin() {
		goto(localizedResolve('/login'));
	}

	$effect(() => {
		const interval = setInterval(() => {
			secondsLeft -= 1;
			if (secondsLeft <= 0) {
				goToLogin();
			}
		}, 1000);

		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>{m.error_title()} • Aerendil</title>
</svelte:head>

<div class="grid min-h-screen place-items-center bg-background p-8 font-sans">
	<div class="w-full max-w-md rounded-xl border border-border bg-surface p-8 text-center">
		<span class="mx-auto mb-4 icon-[lucide--circle-alert] size-10 text-danger" aria-hidden="true"
		></span>
		<p class="mb-1 text-xs font-semibold tracking-widest text-primary uppercase">
			{m.error_status_label({ status: page.status })}
		</p>
		<h1 class="text-xl font-semibold text-foreground">{m.error_title()}</h1>
		<p class="mt-2 text-muted-foreground">{page.error?.message}</p>

		<button
			type="button"
			onclick={goToLogin}
			class="mt-6 w-full cursor-pointer rounded-lg bg-primary px-4 py-3 font-semibold text-primary-foreground hover:bg-primary-hover"
		>
			{m.error_go_login()}
		</button>

		<p class="mt-4 text-sm text-muted-foreground" aria-live="polite">
			{m.error_redirect_countdown({ seconds: secondsLeft })}
		</p>
	</div>
</div>
