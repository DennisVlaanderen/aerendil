<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { uniqueId, type Rule, type SegmentRule } from '#lib/flags.ts';
	import ConditionsEditor from './ConditionsEditor.svelte';
	import ServeEditor from './ServeEditor.svelte';

	// variationIds set means flag rules (description + serve); without it
	// these are segment rules, which have neither and can't use inSegment.
	let {
		rules = $bindable(),
		variationIds,
		segments = null,
		disabled = false
	}: {
		rules: (Rule | SegmentRule)[];
		variationIds?: string[];
		segments?: string[] | null;
		disabled?: boolean;
	} = $props();

	const field =
		'rounded-lg border border-border bg-background px-3 py-1.5 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-60';

	function add() {
		const id = uniqueId(
			'rule',
			rules.map((r) => r.id)
		);
		rules.push(
			variationIds
				? { id, match: 'all', conditions: [], serve: { variation: variationIds[0] } }
				: { id, match: 'all', conditions: [] }
		);
	}

	function move(i: number, by: number) {
		const [rule] = rules.splice(i, 1);
		rules.splice(i + by, 0, rule);
	}
</script>

<div class="grid gap-3">
	{#each rules as rule, i (rule)}
		<section class="grid gap-3 rounded-lg border border-border bg-background p-4">
			<div class="flex flex-wrap items-center gap-2">
				<span class="text-xs font-semibold text-muted-foreground">#{i + 1}</span>
				<input
					class="{field} w-40 font-mono"
					aria-label={m.rule_id_label()}
					bind:value={rule.id}
					required
					{disabled}
				/>
				{#if variationIds && 'serve' in rule}
					<input
						class="{field} min-w-40 flex-1"
						placeholder={m.rule_description_placeholder()}
						aria-label={m.rule_description_placeholder()}
						bind:value={rule.description}
						{disabled}
					/>
				{/if}
				{#if !disabled}
					<div class="ml-auto flex gap-1">
						<button
							type="button"
							class="cursor-pointer rounded p-1.5 text-muted-foreground hover:text-foreground disabled:opacity-40"
							aria-label={m.rule_move_up()}
							disabled={i === 0}
							onclick={() => move(i, -1)}
						>
							<span class="icon-[lucide--arrow-up] size-4" aria-hidden="true"></span>
						</button>
						<button
							type="button"
							class="cursor-pointer rounded p-1.5 text-muted-foreground hover:text-foreground disabled:opacity-40"
							aria-label={m.rule_move_down()}
							disabled={i === rules.length - 1}
							onclick={() => move(i, 1)}
						>
							<span class="icon-[lucide--arrow-down] size-4" aria-hidden="true"></span>
						</button>
						<button
							type="button"
							class="cursor-pointer rounded p-1.5 text-muted-foreground hover:text-danger"
							aria-label={m.rule_remove()}
							onclick={() => rules.splice(i, 1)}
						>
							<span class="icon-[lucide--trash-2] size-4" aria-hidden="true"></span>
						</button>
					</div>
				{/if}
			</div>

			<label class="flex items-center gap-2 text-sm text-foreground">
				<span>{m.rule_match_label()}</span>
				<select class={field} bind:value={rule.match} {disabled}>
					<option value="all">{m.rule_match_all()}</option>
					<option value="any">{m.rule_match_any()}</option>
				</select>
			</label>

			<ConditionsEditor bind:conditions={rule.conditions} {segments} {disabled} />

			{#if variationIds && 'serve' in rule}
				<div class="grid gap-1.5 border-t border-border pt-3">
					<span class="text-sm font-medium text-foreground">{m.rule_serve_label()}</span>
					<ServeEditor bind:serve={rule.serve} {variationIds} {disabled} />
				</div>
			{/if}
		</section>
	{:else}
		<p class="text-sm text-muted-foreground">{m.rules_empty()}</p>
	{/each}
	{#if !disabled}
		<button
			type="button"
			class="cursor-pointer justify-self-start rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-foreground hover:bg-surface-muted"
			onclick={add}
		>
			{m.rule_add()}
		</button>
	{/if}
</div>
