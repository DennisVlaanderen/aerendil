<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import type { Segment } from '#lib/flags.ts';
	import RulesEditor from './RulesEditor.svelte';

	// Shared by the segment create and edit pages; the key is only editable
	// on create. onsave receives the §6.2 body to send.
	let {
		segment,
		creating = false,
		disabled = false,
		saving = false,
		error = '',
		submitLabel,
		onsave
	}: {
		segment: Partial<Segment>;
		creating?: boolean;
		disabled?: boolean;
		saving?: boolean;
		error?: string;
		submitLabel: string;
		onsave: (segment: Partial<Segment>) => void;
	} = $props();

	function toDraft(s: Partial<Segment>) {
		return {
			key: s.key ?? '',
			name: s.name ?? '',
			included: (s.included ?? []).join('\n'),
			excluded: (s.excluded ?? []).join('\n'),
			rules: structuredClone(s.rules ?? [])
		};
	}
	// $state, not a writable $derived: the editors mutate the draft deeply,
	// and the effect resets it to what the backend saved.
	// svelte-ignore state_referenced_locally
	// eslint-disable-next-line svelte/prefer-writable-derived
	let draft = $state(toDraft(segment));
	$effect(() => {
		draft = toDraft(segment);
	});

	const keys = (text: string) =>
		text
			.split('\n')
			.map((line) => line.trim())
			.filter(Boolean);

	function submit(event: SubmitEvent) {
		event.preventDefault();
		onsave({
			key: draft.key.trim(),
			name: draft.name,
			included: keys(draft.included),
			excluded: keys(draft.excluded),
			rules: draft.rules
		});
	}

	const field =
		'w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-60';
</script>

<form onsubmit={submit} class="grid gap-4">
	{#if creating}
		<label class="grid gap-1.5 text-sm font-medium text-foreground">
			<span>{m.segment_key_label()}</span>
			<input class={field} bind:value={draft.key} required {disabled} />
		</label>
	{/if}
	<label class="grid gap-1.5 text-sm font-medium text-foreground">
		<span>{m.segment_name_label()}</span>
		<input class={field} bind:value={draft.name} {disabled} />
	</label>
	<div class="grid gap-4 sm:grid-cols-2">
		<label class="grid gap-1.5 text-sm font-medium text-foreground">
			<span>{m.segment_included_label()}</span>
			<span class="text-xs font-normal text-muted-foreground">{m.segment_keys_hint()}</span>
			<textarea class="{field} font-mono" rows="5" bind:value={draft.included} {disabled}
			></textarea>
		</label>
		<label class="grid gap-1.5 text-sm font-medium text-foreground">
			<span>{m.segment_excluded_label()}</span>
			<span class="text-xs font-normal text-muted-foreground">{m.segment_keys_hint()}</span>
			<textarea class="{field} font-mono" rows="5" bind:value={draft.excluded} {disabled}
			></textarea>
		</label>
	</div>
	<section class="grid gap-2">
		<h2 class="text-sm font-semibold text-foreground">{m.segment_rules_label()}</h2>
		<p class="text-xs text-muted-foreground">{m.segment_rules_hint()}</p>
		<RulesEditor bind:rules={draft.rules} {disabled} />
	</section>

	{#if error}
		<p class="flex items-center gap-2 text-sm text-danger">
			<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
			{error}
		</p>
	{/if}

	{#if !disabled}
		<button
			type="submit"
			disabled={saving}
			class="cursor-pointer justify-self-start rounded-lg bg-primary px-5 py-2.5 font-semibold text-primary-foreground hover:bg-primary-hover disabled:cursor-wait disabled:opacity-70"
		>
			{submitLabel}
		</button>
	{/if}
</form>
