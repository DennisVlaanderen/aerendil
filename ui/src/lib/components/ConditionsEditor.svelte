<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import {
		OPS,
		formatValues,
		parseValues,
		valueKind,
		valueKinds,
		type Condition,
		type ValueKind
	} from '#lib/flags.ts';

	// segments is the inSegment choice list; null hides inSegment, since
	// segment rules must not use it (fqdp.md §6.2).
	let {
		conditions = $bindable(),
		segments,
		disabled = false
	}: { conditions: Condition[]; segments: string[] | null; disabled?: boolean } = $props();

	// Kept per row so an empty values list still remembers the chosen kind.
	let kinds: ValueKind[] = $state(conditions.map(valueKind));

	const ops = $derived(segments ? OPS : OPS.filter((op) => op !== 'inSegment'));
	const field =
		'rounded-lg border border-border bg-background px-3 py-1.5 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-60';

	function setOp(i: number, op: string) {
		const condition = conditions[i];
		condition.op = op;
		if (op === 'inSegment') {
			delete condition.attribute;
		} else {
			condition.attribute ??= '';
		}
		if (!valueKinds(op).includes(kinds[i])) {
			kinds[i] = valueKinds(op)[0];
			condition.values = parseValues(formatValues(condition.values), kinds[i]);
		}
	}

	function setKind(i: number, kind: ValueKind) {
		kinds[i] = kind;
		conditions[i].values = parseValues(formatValues(conditions[i].values), kind);
	}

	function toggleSegment(i: number, key: string, on: boolean) {
		const values = conditions[i].values.filter((v) => v !== key);
		conditions[i].values = on ? [...values, key] : values;
	}

	function add() {
		conditions.push({ attribute: '', op: 'in', values: [] });
		kinds.push('string');
	}

	function remove(i: number) {
		conditions.splice(i, 1);
		kinds.splice(i, 1);
	}
</script>

<div class="grid gap-2">
	{#each conditions as condition, i (condition)}
		<div class="flex flex-wrap items-start gap-2 rounded-lg border border-border bg-surface p-2.5">
			{#if condition.op !== 'inSegment'}
				<input
					class="{field} w-36"
					placeholder={m.condition_attribute_placeholder()}
					aria-label={m.condition_attribute_label()}
					bind:value={condition.attribute}
					required
					{disabled}
				/>
			{/if}
			<select
				class={field}
				aria-label={m.condition_op_label()}
				value={condition.op}
				onchange={(e) => setOp(i, e.currentTarget.value)}
				{disabled}
			>
				{#each ops as op (op)}
					<option value={op}>{op}</option>
				{/each}
			</select>
			{#if valueKinds(condition.op).length > 1}
				<select
					class={field}
					aria-label={m.condition_kind_label()}
					value={kinds[i]}
					onchange={(e) => setKind(i, e.currentTarget.value as ValueKind)}
					{disabled}
				>
					{#each valueKinds(condition.op) as kind (kind)}
						<option value={kind}>{kind}</option>
					{/each}
				</select>
			{/if}
			{#if condition.op === 'inSegment'}
				<fieldset class="flex flex-1 flex-wrap gap-3 py-1.5">
					<legend class="sr-only">{m.condition_segments_label()}</legend>
					<!-- Union, so a referenced segment stays visible even if unlisted. -->
					{#each new Set([...(segments ?? []), ...condition.values.map(String)]) as key (key)}
						<label class="flex items-center gap-1.5 text-sm text-foreground">
							<input
								type="checkbox"
								checked={condition.values.includes(key)}
								onchange={(e) => toggleSegment(i, key, e.currentTarget.checked)}
								{disabled}
							/>
							{key}
						</label>
					{:else}
						<span class="text-sm text-muted-foreground">{m.condition_no_segments()}</span>
					{/each}
				</fieldset>
			{:else}
				<textarea
					class="{field} min-w-40 flex-1"
					rows="2"
					placeholder={m.condition_values_placeholder()}
					aria-label={m.condition_values_label()}
					value={formatValues(condition.values)}
					onchange={(e) => (condition.values = parseValues(e.currentTarget.value, kinds[i]))}
					{disabled}></textarea>
			{/if}
			<label class="flex items-center gap-1.5 py-1.5 text-sm text-foreground">
				<input type="checkbox" bind:checked={condition.negate} {disabled} />
				{m.condition_negate_label()}
			</label>
			{#if !disabled}
				<button
					type="button"
					class="cursor-pointer rounded p-1.5 text-muted-foreground hover:text-danger"
					aria-label={m.condition_remove()}
					onclick={() => remove(i)}
				>
					<span class="icon-[lucide--x] size-4" aria-hidden="true"></span>
				</button>
			{/if}
		</div>
	{/each}
	{#if !disabled}
		<button
			type="button"
			class="cursor-pointer justify-self-start text-sm font-medium text-primary hover:underline"
			onclick={add}
		>
			{m.condition_add()}
		</button>
	{/if}
</div>
