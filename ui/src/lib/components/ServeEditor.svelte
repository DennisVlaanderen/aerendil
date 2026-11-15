<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { ROLLOUT_SCALE, evenWeights, totalWeight, type Serve } from '#lib/flags.ts';

	let {
		serve = $bindable(),
		variationIds,
		disabled = false
	}: { serve: Serve; variationIds: string[]; disabled?: boolean } = $props();

	const field =
		'rounded-lg border border-border bg-background px-3 py-1.5 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-60';
	const name = $props.id();

	const total = $derived(serve.rollout ? totalWeight(serve.rollout.weights) : 0);

	function useVariation() {
		serve.variation = serve.rollout?.weights[0]?.variation ?? variationIds[0];
		delete serve.rollout;
	}

	function useRollout() {
		serve.rollout = { weights: evenWeights(variationIds) };
		delete serve.variation;
	}

	function weightOf(id: string) {
		return serve.rollout?.weights.find((w) => w.variation === id)?.weight ?? 0;
	}

	// Rebuilt from the current variations so a renamed or removed variation
	// never lingers in the weights; the input is a percentage, stored on
	// the 100000 scale (§6.5).
	function setWeight(id: string, percent: number) {
		if (!serve.rollout) return;
		const weight = Math.round((Number.isFinite(percent) ? percent : 0) * (ROLLOUT_SCALE / 100));
		serve.rollout.weights = variationIds.map((variation) => ({
			variation,
			weight: variation === id ? weight : weightOf(variation)
		}));
	}
</script>

<div class="grid gap-2">
	<div class="flex flex-wrap gap-4 text-sm text-foreground">
		<label class="flex items-center gap-1.5">
			<input
				type="radio"
				name="{name}-kind"
				checked={!serve.rollout}
				onchange={useVariation}
				{disabled}
			/>
			{m.serve_variation()}
		</label>
		<label class="flex items-center gap-1.5">
			<input
				type="radio"
				name="{name}-kind"
				checked={!!serve.rollout}
				onchange={useRollout}
				{disabled}
			/>
			{m.serve_rollout()}
		</label>
	</div>

	{#if serve.rollout}
		<div class="grid gap-2">
			{#each variationIds as id (id)}
				<label class="flex items-center gap-2 text-sm text-foreground">
					<span class="w-32 truncate font-mono">{id}</span>
					<input
						type="number"
						min="0"
						max="100"
						step="0.001"
						class="{field} w-28"
						value={weightOf(id) / (ROLLOUT_SCALE / 100)}
						onchange={(e) => setWeight(id, e.currentTarget.valueAsNumber)}
						{disabled}
					/>
					<span class="text-muted-foreground">%</span>
				</label>
			{/each}
			<p class="text-sm {total === ROLLOUT_SCALE ? 'text-muted-foreground' : 'text-danger'}">
				{m.serve_rollout_total({
					percent: (total / (ROLLOUT_SCALE / 100)).toFixed(3),
					weight: total
				})}
			</p>
			<div class="flex flex-wrap items-center gap-3">
				<label class="flex items-center gap-2 text-sm text-foreground">
					<span>{m.serve_bucket_by_label()}</span>
					<input
						class="{field} w-40"
						placeholder="targetingKey"
						bind:value={serve.rollout.bucketBy}
						{disabled}
					/>
				</label>
				{#if !disabled}
					<button
						type="button"
						class="cursor-pointer text-sm font-medium text-primary hover:underline"
						onclick={useRollout}
					>
						{m.serve_split_evenly()}
					</button>
				{/if}
			</div>
		</div>
	{:else}
		<select class="{field} justify-self-start" bind:value={serve.variation} {disabled}>
			{#each variationIds as id (id)}
				<option value={id}>{id}</option>
			{/each}
		</select>
	{/if}
</div>
