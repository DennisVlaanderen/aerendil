<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { uniqueId, type FlagType } from '#lib/flags.ts';

	// Values are edited as text and parsed on save (parseVariationValue); a
	// boolean flag's two variations are fixed (fqdp.md §6.1). A saved
	// variation's id is locked: offVariation, rules and rollouts refer to it.
	let {
		variations = $bindable(),
		type,
		disabled = false
	}: {
		variations: { id: string; text: string; saved?: boolean }[];
		type: FlagType;
		disabled?: boolean;
	} = $props();

	const field =
		'rounded-lg border border-border bg-background px-3 py-1.5 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-60';
	const fixed = $derived(disabled || type === 'boolean');
</script>

<div class="grid gap-2">
	{#each variations as variation, i (variation)}
		<div class="flex flex-wrap items-start gap-2">
			<input
				class="{field} w-40 font-mono"
				aria-label={m.variation_id_label()}
				bind:value={variation.id}
				required
				disabled={fixed || variation.saved}
			/>
			{#if type === 'json'}
				<textarea
					class="{field} min-w-48 flex-1 font-mono"
					rows="3"
					aria-label={m.variation_value_label()}
					bind:value={variation.text}
					{disabled}></textarea>
			{:else}
				<input
					class="{field} min-w-48 flex-1"
					type={type === 'integer' ? 'number' : 'text'}
					step="1"
					aria-label={m.variation_value_label()}
					bind:value={variation.text}
					disabled={fixed}
				/>
			{/if}
			{#if !fixed && variations.length > 1}
				<button
					type="button"
					class="cursor-pointer rounded p-1.5 text-muted-foreground hover:text-danger"
					aria-label={m.variation_remove()}
					onclick={() => variations.splice(i, 1)}
				>
					<span class="icon-[lucide--x] size-4" aria-hidden="true"></span>
				</button>
			{/if}
		</div>
	{/each}
	{#if !fixed}
		<button
			type="button"
			class="cursor-pointer justify-self-start text-sm font-medium text-primary hover:underline"
			onclick={() =>
				variations.push({
					id: uniqueId(
						'variation',
						variations.map((v) => v.id)
					),
					text: type === 'json' ? '{}' : ''
				})}
		>
			{m.variation_add()}
		</button>
	{/if}
</div>
