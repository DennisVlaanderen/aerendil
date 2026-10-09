<script lang="ts">
	import { goto } from '$app/navigation';
	import { apiRequest } from '#lib/client/api.ts';
	import { resolveErrorMessage } from '#lib/errors.ts';
	import LocaleSwitcher from '#lib/components/LocaleSwitcher.svelte';
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { m } from '#lib/paraglide/messages.js';

	let isSubmitting = $state(false);
	let errorMessage = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		const formEl = event.currentTarget as HTMLFormElement;
		const data = new FormData(formEl);

		isSubmitting = true;
		errorMessage = '';

		const result = await apiRequest('/bff/login', {
			method: 'POST',
			body: JSON.stringify({
				username: (data.get('username') ?? '').toString(),
				password: (data.get('password') ?? '').toString()
			})
		});

		if (result.error) {
			errorMessage = resolveErrorMessage(result.code);
			isSubmitting = false;
			return;
		}

		await goto(localizedResolve('/dashboard'));
	}
</script>

<svelte:head>
	<title>Login • Aerendil</title>
</svelte:head>

<div class="fixed top-6 right-6 z-20">
	<LocaleSwitcher />
</div>

<div class="grid min-h-screen place-items-center bg-background p-8 font-sans">
	<div class="w-full max-w-md rounded-xl border border-border bg-surface p-8">
		<img src="/aerendil-logo.svg" class="mb-6 h-8 w-auto dark:hidden" alt="Aerendil" />
		<img src="/aerendil-logo-dark.svg" class="mb-6 hidden h-8 w-auto dark:block" alt="Aerendil" />
		<div class="mb-6">
			<p class="mb-1 text-xs font-semibold tracking-widest text-primary uppercase">
				{m.login_eyebrow()}
			</p>
			<h1 class="text-2xl font-semibold text-foreground">{m.login_title()}</h1>
			<p class="mt-1 text-muted-foreground">{m.login_subtitle()}</p>
		</div>

		<form method="POST" class="grid gap-4" onsubmit={handleSubmit}>
			<label class="grid gap-1.5 text-sm font-medium text-foreground">
				<span>{m.login_username_label()}</span>
				<div class="relative">
					<span
						class="absolute top-1/2 left-3.5 icon-[lucide--user] size-4 -translate-y-1/2 text-muted-foreground"
						aria-hidden="true"
					></span>
					<input
						name="username"
						type="text"
						autocomplete="username"
						required
						class="w-full rounded-lg border border-border bg-background py-3 pr-4 pl-10 text-base text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
					/>
				</div>
			</label>

			<label class="grid gap-1.5 text-sm font-medium text-foreground">
				<span>{m.login_password_label()}</span>
				<div class="relative">
					<span
						class="absolute top-1/2 left-3.5 icon-[lucide--lock] size-4 -translate-y-1/2 text-muted-foreground"
						aria-hidden="true"
					></span>
					<input
						name="password"
						type="password"
						autocomplete="current-password"
						required
						class="w-full rounded-lg border border-border bg-background py-3 pr-4 pl-10 text-base text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
					/>
				</div>
			</label>

			{#if errorMessage}
				<p class="flex items-center gap-2 text-sm text-danger">
					<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
					{errorMessage}
				</p>
			{/if}

			<button
				type="submit"
				disabled={isSubmitting}
				class="cursor-pointer rounded-lg bg-primary px-4 py-3.5 font-semibold text-primary-foreground hover:bg-primary-hover disabled:cursor-wait disabled:opacity-70"
			>
				{isSubmitting ? m.login_submitting() : m.login_submit()}
			</button>
		</form>

		<div class="mt-5 border-t border-border pt-4 text-sm text-muted-foreground">
			<p>{m.login_demo_hint()}</p>
			<p>{m.login_demo_admin()}</p>
		</div>
	</div>
</div>
