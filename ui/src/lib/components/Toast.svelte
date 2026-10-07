<script lang="ts">
	import { toast } from '#lib/toast.svelte.ts';
	import { m } from '#lib/paraglide/messages.js';
</script>

{#if toast.message}
	<div
		class="fixed inset-x-0 bottom-6 z-40 flex justify-center px-4"
		role="status"
		aria-live="polite"
	>
		<div
			class="flex max-w-md items-center gap-3 rounded-xl border border-line-1 bg-surface px-4 py-3 text-sm font-medium text-ink"
		>
			<span class="icon-[lucide--info] size-4 shrink-0 text-ink-muted" aria-hidden="true"></span>
			<p class="flex-1">{toast.message}</p>
			<button
				type="button"
				class="icon-[lucide--x] size-4 shrink-0 cursor-pointer text-ink-muted hover:text-ink"
				aria-label={m.toast_dismiss()}
				onclick={() => toast.dismiss()}
			></button>
		</div>
	</div>
{/if}
