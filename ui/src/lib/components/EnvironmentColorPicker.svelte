<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { ENVIRONMENT_COLORS } from '#lib/environment-colors.ts';

	// Submits under `name`: a palette key ('1'-'6'), a custom hex, or '' for no color.
	interface Props {
		name: string;
		label: string;
		value?: string;
	}

	let { name, label, value = $bindable('') }: Props = $props();

	const id = $props.id();
	let isCustom = $derived(value.startsWith('#'));
	// <input type="color"> only accepts #rrggbb, so expand the short #rgb form.
	let customHex = $derived(
		!isCustom
			? '#6b7280'
			: value.length === 4
				? `#${[...value.slice(1)].map((c) => c + c).join('')}`
				: value
	);

	const swatch =
		'flex size-7 items-center justify-center rounded-full border border-border peer-checked:ring-2 peer-checked:ring-ring peer-checked:ring-offset-2 peer-checked:ring-offset-surface peer-focus-visible:outline-2 peer-focus-visible:outline-offset-4 peer-focus-visible:outline-ring';
</script>

<fieldset class="grid gap-1.5">
	<legend class="mb-1.5 text-sm font-medium text-foreground">{label}</legend>
	<input type="hidden" {name} {value} />

	<div class="flex flex-wrap items-center gap-3">
		<label class="cursor-pointer" title={m.environments_color_none()}>
			<input type="radio" name="{id}-swatch" value="" bind:group={value} class="peer sr-only" />
			<span class="{swatch} bg-background text-muted-foreground">
				<span class="icon-[lucide--ban] size-4" aria-hidden="true"></span>
			</span>
			<span class="sr-only">{m.environments_color_none()}</span>
		</label>

		{#each Object.entries(ENVIRONMENT_COLORS) as [key, hex] (key)}
			<label class="cursor-pointer" title={m.environments_color_preset({ number: key })}>
				<input
					type="radio"
					name="{id}-swatch"
					value={key}
					bind:group={value}
					class="peer sr-only"
				/>
				<span class={swatch} style:background-color={hex}></span>
				<span class="sr-only">{m.environments_color_preset({ number: key })}</span>
			</label>
		{/each}

		<label class="relative cursor-pointer" title={m.environments_color_custom()}>
			<input
				type="color"
				value={customHex}
				oninput={(event) => (value = event.currentTarget.value)}
				aria-label={m.environments_color_custom()}
				class="peer absolute inset-0 size-full cursor-pointer opacity-0"
			/>
			<span
				class="{swatch} {isCustom
					? 'ring-2 ring-ring ring-offset-2 ring-offset-surface'
					: ''} text-white"
				style:background={isCustom
					? customHex
					: 'conic-gradient(#ef4444, #eab308, #22c55e, #3b82f6, #8b5cf6, #ef4444)'}
			>
				<span class="icon-[lucide--pipette] size-3.5 drop-shadow" aria-hidden="true"></span>
			</span>
		</label>
	</div>
</fieldset>
