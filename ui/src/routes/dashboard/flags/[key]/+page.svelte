<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { apiRequest } from '#lib/client/api.ts';
	import { resolveErrorMessage } from '#lib/errors.ts';
	import { hasPermission } from '#lib/permissions.ts';
	import { localizedResolve } from '#lib/localizedResolve.ts';
	import { m } from '#lib/paraglide/messages.js';
	import ConfirmModal from '#lib/components/ConfirmModal.svelte';
	import VariationsEditor from '#lib/components/VariationsEditor.svelte';
	import RulesEditor from '#lib/components/RulesEditor.svelte';
	import ServeEditor from '#lib/components/ServeEditor.svelte';
	import {
		parseVariationValue,
		variationText,
		type EvaluationResult,
		type FlagDefinition
	} from '#lib/flags.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// data.environments/selectedEnvironmentId cascade down from
	// dashboard/+layout.server.ts -- the same key can now exist
	// independently per environment, so the detail view needs to say which
	// one it's showing.
	let selectedEnvironmentName = $derived(
		data.environments.find((e) => e.id === data.selectedEnvironmentId)?.name
	);

	const canUpdate = $derived(hasPermission(data, 'flags:update'));
	const canDelete = $derived(hasPermission(data, 'flags:delete'));

	let isSaving = $state(false);
	let saveError = $state('');

	// The editable copy; variation values are text until saved.
	function toDraft(flag: FlagDefinition) {
		const copy = structuredClone(flag);
		return {
			enabled: copy.enabled,
			variations: copy.variations.map((v) => ({
				id: v.id,
				text: variationText(v.value, flag.type)
			})),
			offVariation: copy.offVariation,
			rules: copy.rules ?? [],
			fallthrough: copy.fallthrough
		};
	}
	// $state, not a writable $derived: the editors mutate the draft deeply,
	// and the effect resets it to what the backend saved.
	// svelte-ignore state_referenced_locally
	// eslint-disable-next-line svelte/prefer-writable-derived
	let draft = $state(toDraft(data.flag));
	$effect(() => {
		draft = toDraft(data.flag);
	});
	const variationIds = $derived(draft.variations.map((v) => v.id));

	let targetingKey = $state('');
	let attributes = $state('{}');
	let isEvaluating = $state(false);
	let evaluateError = $state('');
	let evaluation: EvaluationResult | undefined = $state();

	let deleteModal: { show: () => void } | undefined = $state();
	let isDeleting = $state(false);
	let deleteError = $state('');

	function flagUrl() {
		return `/bff/flags/${encodeURIComponent(data.flag.key)}?environmentId=${encodeURIComponent(data.flag.environmentId)}`;
	}

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		saveError = '';
		let variations;
		try {
			variations = draft.variations.map((v) => ({
				id: v.id,
				value: parseVariationValue(v.text, data.flag.type)
			}));
		} catch {
			saveError = m.flag_variation_value_parse_error();
			return;
		}

		isSaving = true;
		const result = await apiRequest(flagUrl(), {
			method: 'PUT',
			body: JSON.stringify({
				key: data.flag.key,
				type: data.flag.type,
				enabled: draft.enabled,
				variations,
				offVariation: draft.offVariation,
				rules: draft.rules,
				fallthrough: draft.fallthrough
			})
		});

		isSaving = false;
		if (result.error) {
			saveError = resolveErrorMessage(result.code);
			return;
		}
		await invalidateAll();
	}

	async function handleEvaluate(event: SubmitEvent) {
		event.preventDefault();
		evaluateError = '';
		evaluation = undefined;
		let parsed;
		try {
			parsed = JSON.parse(attributes || '{}');
		} catch {
			evaluateError = m.flag_evaluate_attributes_invalid();
			return;
		}

		isEvaluating = true;
		const result = await apiRequest<EvaluationResult>(
			`/bff/flags/${encodeURIComponent(data.flag.key)}/evaluate?environmentId=${encodeURIComponent(data.flag.environmentId)}`,
			{ method: 'POST', body: JSON.stringify({ targetingKey, attributes: parsed }) }
		);
		isEvaluating = false;
		if (result.error) {
			evaluateError = resolveErrorMessage(result.code);
			return;
		}
		evaluation = result.data;
	}

	async function confirmDelete() {
		isDeleting = true;
		deleteError = '';

		const result = await apiRequest(flagUrl(), { method: 'DELETE' });

		isDeleting = false;
		if (result.error) {
			deleteError = resolveErrorMessage(result.code);
			return;
		}
		await goto(localizedResolve('/dashboard'));
	}
</script>

<svelte:head>
	<title>{data.flag.key} • Aerendil</title>
</svelte:head>

<div class="p-7">
	<div class="w-full max-w-4xl rounded-xl border border-border bg-surface p-6">
		<p class="mb-1 text-xs font-semibold tracking-widest text-primary uppercase">
			{m.nav_flags()}
		</p>
		<h1 class="mb-1 flex items-center gap-3 text-xl font-semibold break-words text-foreground">
			<span class="icon-[lucide--flag] size-6 shrink-0 text-primary" aria-hidden="true"></span>
			{data.flag.key}
		</h1>
		<p class="mb-5 text-sm text-muted-foreground">
			{selectedEnvironmentName ? m.flag_detail_environment({ name: selectedEnvironmentName }) : ''}
		</p>

		<div class="grid grid-cols-[repeat(auto-fit,minmax(12rem,1fr))] gap-4">
			<div class="grid gap-1.5 rounded-lg border border-border bg-background p-4">
				<span class="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
					>{m.flag_detail_version()}</span
				>
				<strong class="text-foreground">{data.flag.revision}</strong>
			</div>
			<div class="grid gap-1.5 rounded-lg border border-border bg-background p-4">
				<span class="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
					>{m.flag_detail_type()}</span
				>
				<strong class="text-foreground">{data.flag.type}</strong>
			</div>
		</div>

		<form onsubmit={handleSave} class="mt-4 grid gap-4">
			<label class="flex items-center gap-1.5 text-sm text-foreground">
				<input type="checkbox" bind:checked={draft.enabled} disabled={!canUpdate} />
				{m.flag_edit_enabled_label()}
			</label>

			<section class="grid gap-2">
				<h2 class="text-sm font-semibold text-foreground">{m.flag_variations_label()}</h2>
				<VariationsEditor
					bind:variations={draft.variations}
					type={data.flag.type}
					disabled={!canUpdate}
				/>
			</section>

			<label class="grid gap-1.5 text-sm text-foreground">
				<span class="font-semibold">{m.flag_off_variation_label()}</span>
				<span class="text-xs text-muted-foreground">{m.flag_off_variation_hint()}</span>
				<select
					bind:value={draft.offVariation}
					disabled={!canUpdate}
					class="justify-self-start rounded-lg border border-border bg-background px-3 py-1.5 text-sm text-foreground disabled:opacity-60"
				>
					{#each variationIds as id (id)}
						<option value={id}>{id}</option>
					{/each}
				</select>
			</label>

			<section class="grid gap-2">
				<h2 class="text-sm font-semibold text-foreground">{m.flag_rules_label()}</h2>
				<p class="text-xs text-muted-foreground">{m.flag_rules_hint()}</p>
				<RulesEditor
					bind:rules={draft.rules}
					{variationIds}
					segments={data.segmentKeys}
					disabled={!canUpdate}
				/>
			</section>

			<section class="grid gap-2">
				<h2 class="text-sm font-semibold text-foreground">{m.flag_fallthrough_label()}</h2>
				<p class="text-xs text-muted-foreground">{m.flag_fallthrough_hint()}</p>
				<ServeEditor bind:serve={draft.fallthrough} {variationIds} disabled={!canUpdate} />
			</section>

			{#if saveError}
				<p class="flex items-center gap-2 text-sm text-danger">
					<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
					{saveError}
				</p>
			{/if}

			{#if canUpdate}
				<button
					type="submit"
					disabled={isSaving}
					class="cursor-pointer justify-self-start rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-surface-muted disabled:cursor-wait disabled:opacity-70"
				>
					{m.flag_edit_submit()}
				</button>
			{/if}
		</form>

		<form onsubmit={handleEvaluate} class="mt-6 grid gap-3 border-t border-border pt-5">
			<h2 class="text-sm font-semibold text-foreground">{m.flag_evaluate_title()}</h2>
			<p class="text-xs text-muted-foreground">{m.flag_evaluate_hint()}</p>
			<label class="grid gap-1.5 text-sm text-foreground">
				<span>{m.flag_evaluate_targeting_key_label()}</span>
				<input
					bind:value={targetingKey}
					required
					class="rounded-lg border border-border bg-background px-3 py-1.5 text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
				/>
			</label>
			<label class="grid gap-1.5 text-sm text-foreground">
				<span>{m.flag_evaluate_attributes_label()}</span>
				<textarea
					bind:value={attributes}
					rows="3"
					class="rounded-lg border border-border bg-background px-3 py-1.5 font-mono text-sm text-foreground focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
				></textarea>
			</label>
			{#if evaluateError}
				<p class="flex items-center gap-2 text-sm text-danger">
					<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
					{evaluateError}
				</p>
			{/if}
			{#if evaluation}
				<dl
					class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-lg border border-border bg-background p-3 text-sm"
				>
					<dt class="text-muted-foreground">{m.flag_evaluate_value()}</dt>
					<dd class="font-mono break-all text-foreground">{JSON.stringify(evaluation.value)}</dd>
					<dt class="text-muted-foreground">{m.flag_evaluate_variation()}</dt>
					<dd class="font-mono text-foreground">{evaluation.variationId ?? '—'}</dd>
					<dt class="text-muted-foreground">{m.flag_evaluate_reason()}</dt>
					<dd class="font-mono text-foreground">
						{evaluation.reason}{evaluation.ruleId
							? ` (${evaluation.ruleId})`
							: ''}{evaluation.errorKind ? ` — ${evaluation.errorKind}` : ''}
					</dd>
				</dl>
			{/if}
			<button
				type="submit"
				disabled={isEvaluating}
				class="cursor-pointer justify-self-start rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-surface-muted disabled:cursor-wait disabled:opacity-70"
			>
				{m.flag_evaluate_submit()}
			</button>
		</form>

		{#if canDelete}
			<div class="mt-6 border-t border-border pt-5">
				{#if deleteError}
					<p class="mb-3 flex items-center gap-2 text-sm text-danger">
						<span class="icon-[lucide--circle-alert] size-4 shrink-0" aria-hidden="true"></span>
						{deleteError}
					</p>
				{/if}
				<button
					type="button"
					disabled={isDeleting}
					class="cursor-pointer text-sm font-medium text-danger hover:underline disabled:cursor-wait disabled:opacity-70"
					onclick={() => deleteModal?.show()}
				>
					{m.flag_delete_button()}
				</button>
			</div>
		{/if}
	</div>
</div>

<ConfirmModal
	bind:this={deleteModal}
	title={m.flag_delete_confirm_title()}
	description={m.flag_delete_confirm_description({ key: data.flag.key })}
	confirmLabel={m.flag_delete_button()}
	cancelLabel={m.modal_cancel()}
	variant="danger"
	onconfirm={confirmDelete}
/>
