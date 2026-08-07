<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import LocaleSwitcher from '$lib/components/LocaleSwitcher.svelte';
	import { localizeHref } from '$lib/paraglide/runtime';
	import { m } from '$lib/paraglide/messages.js';
	import type { Pathname } from '$app/types';

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

		await goto(resolve(localizeHref('/dashboard') as Pathname));
	}
</script>

<svelte:head>
	<title>Login • Aerendil</title>
</svelte:head>

<div class="fixed top-6 right-6 z-20">
	<LocaleSwitcher />
</div>

<div class="grid min-h-screen place-items-center bg-page p-8 font-sans">
	<div class="w-full max-w-md rounded-xl border border-line-1 bg-surface p-8">
		<div class="mb-6">
			<p class="mb-1 text-xs font-semibold tracking-widest text-nav-active uppercase">
				{m.login_eyebrow()}
			</p>
			<h1 class="text-2xl font-semibold text-ink">{m.login_title()}</h1>
			<p class="mt-1 text-ink-muted">{m.login_subtitle()}</p>
		</div>

		<form method="POST" class="grid gap-4" onsubmit={handleSubmit}>
			<label class="grid gap-1.5 text-sm font-medium text-ink">
				<span>{m.login_username_label()}</span>
				<div class="relative">
					<span
						class="icon-[lucide--user] absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-ink-muted"
						aria-hidden="true"
					></span>
					<input
						name="username"
						type="text"
						autocomplete="username"
						required
						class="w-full rounded-lg border border-line-1 bg-page py-3 pr-4 pl-10 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
					/>
				</div>
			</label>

			<label class="grid gap-1.5 text-sm font-medium text-ink">
				<span>{m.login_password_label()}</span>
				<div class="relative">
					<span
						class="icon-[lucide--lock] absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-ink-muted"
						aria-hidden="true"
					></span>
					<input
						name="password"
						type="password"
						autocomplete="current-password"
						required
						class="w-full rounded-lg border border-line-1 bg-page py-3 pr-4 pl-10 text-base text-ink focus:border-gold focus:ring-2 focus:ring-gold/40 focus:outline-none"
					/>
				</div>
			</label>

			{#if errorMessage}
				<p class="flex items-center gap-2 text-sm text-error">
					<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
					{errorMessage}
				</p>
			{/if}

			<button
				type="submit"
				disabled={isSubmitting}
				class="cursor-pointer rounded-lg bg-gold px-4 py-3.5 font-semibold text-navy hover:opacity-90 disabled:cursor-wait disabled:opacity-70"
			>
				{isSubmitting ? m.login_submitting() : m.login_submit()}
			</button>
		</form>

		<div class="mt-5 border-t border-line-2 pt-4 text-sm text-ink-muted">
			<p>{m.login_demo_hint()}</p>
			<p>{m.login_demo_admin()}</p>
		</div>
	</div>
</div>
