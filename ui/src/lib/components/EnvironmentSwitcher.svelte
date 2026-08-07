<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { apiRequest } from '$lib/client/api';
	import { resolveErrorMessage } from '$lib/errors';
	import { toast } from '$lib/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { EnvironmentSummary } from '$lib/server/environments';

	let {
		environments,
		selectedEnvironmentId
	}: {
		environments: EnvironmentSummary[];
		selectedEnvironmentId: string | undefined;
	} = $props();

	let open = $state(false);
	let container: HTMLDivElement | undefined = $state();

	// Same custom button+listbox pattern as LocaleSwitcher.svelte -- a native
	// <select> can't be restyled consistently across browsers (especially
	// its open dropdown), so every other selector-style control in this app
	// already avoids it in favor of this pattern.
	const selected = $derived(environments.find((e) => e.id === selectedEnvironmentId));

	function toggleOpen() {
		open = !open;
	}

	async function choose(environmentId: string) {
		open = false;
		if (environmentId === selectedEnvironmentId) return;

		const result = await apiRequest('/bff/selected-environment', {
			method: 'POST',
			body: JSON.stringify({ environmentId })
		});
		if (result.error) {
			toast.show(resolveErrorMessage(result.code));
			return;
		}
		await invalidateAll();
	}

	function handleClickOutside(event: MouseEvent) {
		if (open && container && !container.contains(event.target as Node)) {
			open = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			open = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} onkeydown={handleKeydown} />

{#if environments.length > 0}
	<div class="relative flex" bind:this={container}>
		<button
			type="button"
			class="flex cursor-pointer items-center gap-2 rounded-lg border border-line-1 bg-page px-2.5 py-2.25 text-[13.5px] font-medium whitespace-nowrap text-ink"
			aria-haspopup="listbox"
			aria-expanded={open}
			aria-label={m.header_environment_select_label()}
			onclick={toggleOpen}
		>
			<span class="icon-[lucide--layers] size-3.5 text-ink-muted" aria-hidden="true"></span>
			<span>{selected?.name ?? ''}</span>
			<span
				class="icon-[lucide--chevron-down] size-3.5 text-ink-muted transition-transform duration-150 {open
					? 'rotate-180'
					: ''}"
				aria-hidden="true"
			></span>
		</button>

		{#if open}
			<ul
				class="absolute top-[calc(100%+0.4rem)] right-0 z-30 flex min-w-40 flex-col gap-0.5 rounded-xl border border-line-1 bg-surface p-1.5"
				role="listbox"
				aria-label={m.header_environment_select_label()}
			>
				{#each environments as environment (environment.id)}
					<li>
						<button
							type="button"
							class="flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium text-ink hover:bg-line-3 {environment.id ===
							selectedEnvironmentId
								? 'bg-nav-active-bg text-nav-active'
								: ''}"
							role="option"
							aria-selected={environment.id === selectedEnvironmentId}
							onclick={() => choose(environment.id)}
						>
							{environment.name}
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
{/if}
