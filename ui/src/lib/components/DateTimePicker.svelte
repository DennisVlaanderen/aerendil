<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	// Edits in local time, submits RFC 3339 UTC under `name`. `value`, `min`,
	// `max` are RFC 3339 ('' = unset). Custom calendar since the native one can't be themed.
	interface Props {
		name: string;
		label: string;
		value?: string;
		min?: string;
		max?: string;
	}

	let { name, label, value = $bindable(''), min = '', max = '' }: Props = $props();

	const id = $props.id();
	let open = $state(false);
	let root: HTMLDivElement | undefined = $state();
	let trigger: HTMLButtonElement | undefined = $state();

	function parse(iso: string): Date | null {
		const date = new Date(iso);
		return iso && !Number.isNaN(date.getTime()) ? date : null;
	}

	let selected = $derived(parse(value));
	let minDate = $derived(parse(min));
	let maxDate = $derived(parse(max));

	// The month the calendar shows; follows the selection until navigated.
	let viewMonth = $derived.by(() => {
		const base = selected ?? new Date();
		return new Date(base.getFullYear(), base.getMonth(), 1);
	});

	const pad = (n: number) => String(n).padStart(2, '0');
	let time = $derived(
		selected ? `${pad(selected.getHours())}:${pad(selected.getMinutes())}` : '00:00'
	);
	let [hours, minutes] = $derived(time.split(':'));

	// Text fields, not <input type="time"> (unthemeable). Typing clamps; arrows wrap.
	function setTimePart(
		part: 'hours' | 'minutes',
		input: HTMLInputElement,
		next: number,
		wrap = false
	) {
		const size = part === 'hours' ? 24 : 60;
		if (!selected || Number.isNaN(next)) {
			input.value = part === 'hours' ? hours : minutes;
			return;
		}
		const n = wrap
			? ((next % size) + size) % size
			: Math.min(size - 1, Math.max(0, Math.trunc(next)));
		commit(selected, part === 'hours' ? `${pad(n)}:${minutes}` : `${hours}:${pad(n)}`);
		input.value = pad(n);
	}

	function stepTimePart(
		part: 'hours' | 'minutes',
		event: KeyboardEvent & { currentTarget: HTMLInputElement }
	) {
		const step = { ArrowUp: 1, ArrowDown: -1 }[event.key];
		if (!step) return;
		event.preventDefault();
		setTimePart(part, event.currentTarget, Number(event.currentTarget.value) + step, true);
	}

	let displayValue = $derived(
		selected
			? new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(
					selected
				)
			: m.datetime_picker_unset()
	);
	let monthLabel = $derived(
		new Intl.DateTimeFormat(getLocale(), { month: 'long', year: 'numeric' }).format(viewMonth)
	);
	// Monday-first; 2024-01-01 was a Monday.
	const weekdays = Array.from({ length: 7 }, (_, i) =>
		new Intl.DateTimeFormat(getLocale(), { weekday: 'narrow' }).format(new Date(2024, 0, 1 + i))
	);

	// Leading nulls pad the first week so day 1 lands under its weekday.
	let days = $derived.by(() => {
		const year = viewMonth.getFullYear();
		const month = viewMonth.getMonth();
		const leading = (viewMonth.getDay() + 6) % 7;
		const count = new Date(year, month + 1, 0).getDate();
		return [
			...Array<null>(leading).fill(null),
			...Array.from({ length: count }, (_, i) => new Date(year, month, i + 1))
		];
	});

	const sameDay = (a: Date | null, b: Date) =>
		!!a &&
		a.getFullYear() === b.getFullYear() &&
		a.getMonth() === b.getMonth() &&
		a.getDate() === b.getDate();

	function outOfRange(day: Date): boolean {
		const endOfDay = new Date(day.getFullYear(), day.getMonth(), day.getDate(), 23, 59);
		return (!!minDate && endOfDay < minDate) || (!!maxDate && day > maxDate);
	}

	// Clamped to min/max so a picked time on a boundary day can't cross it.
	function commit(day: Date, hhmm: string) {
		const [hours, minutes] = hhmm.split(':').map(Number);
		let date = new Date(day.getFullYear(), day.getMonth(), day.getDate(), hours || 0, minutes || 0);
		if (minDate && date < minDate) date = minDate;
		if (maxDate && date > maxDate) date = maxDate;
		value = date.toISOString().replace(/\.\d{3}Z$/, 'Z');
	}

	function shiftMonth(delta: number) {
		viewMonth = new Date(viewMonth.getFullYear(), viewMonth.getMonth() + delta, 1);
	}

	// Refocus on keyboard/Done close; an outside click already moved focus.
	function close(refocus = true) {
		open = false;
		if (refocus) trigger?.focus();
	}
</script>

<svelte:window
	onpointerdown={(event) => {
		if (open && !root?.contains(event.target as Node)) close(false);
	}}
	onkeydown={(event) => {
		if (open && event.key === 'Escape') close();
	}}
/>

<div bind:this={root} class="relative grid gap-1.5 text-sm text-foreground">
	<span id="{id}-label" class="font-medium">{label}</span>
	<div class="flex items-center gap-1">
		<button
			type="button"
			aria-labelledby="{id}-label {id}-value"
			bind:this={trigger}
			aria-haspopup="dialog"
			aria-expanded={open}
			onclick={() => (open = !open)}
			class="flex min-w-48 cursor-pointer items-center justify-between gap-3 rounded-lg border border-border bg-background px-4 py-2 text-left text-sm focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none"
		>
			<span id="{id}-value" class={selected ? 'text-foreground' : 'text-muted-foreground'}>
				{displayValue}
			</span>
			<span class="icon-[lucide--calendar] size-4 text-muted-foreground" aria-hidden="true"></span>
		</button>
		{#if selected}
			<button
				type="button"
				aria-label={m.datetime_picker_clear()}
				onclick={() => (value = '')}
				class="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-surface-muted"
			>
				<span class="icon-[lucide--x] size-4" aria-hidden="true"></span>
			</button>
		{/if}
	</div>
	<input type="hidden" {name} {value} />

	{#if open}
		<div
			role="dialog"
			aria-label={label}
			class="absolute top-full left-0 z-20 mt-1 grid w-72 gap-3 rounded-xl border border-border bg-surface p-4 shadow-lg"
		>
			<div class="flex items-center justify-between">
				<button
					type="button"
					aria-label={m.datetime_picker_previous_month()}
					onclick={() => shiftMonth(-1)}
					class="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-surface-muted"
				>
					<span class="icon-[lucide--chevron-left] size-4" aria-hidden="true"></span>
				</button>
				<span class="font-semibold capitalize">{monthLabel}</span>
				<button
					type="button"
					aria-label={m.datetime_picker_next_month()}
					onclick={() => shiftMonth(1)}
					class="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-surface-muted"
				>
					<span class="icon-[lucide--chevron-right] size-4" aria-hidden="true"></span>
				</button>
			</div>

			<div class="grid grid-cols-7 gap-1 text-center">
				{#each weekdays as weekday, i (i)}
					<span class="py-1 text-xs font-semibold text-muted-foreground uppercase">{weekday}</span>
				{/each}
				{#each days as day, i (day?.getTime() ?? `pad-${i}`)}
					{#if day}
						{@const isSelected = sameDay(selected, day)}
						<button
							type="button"
							disabled={outOfRange(day)}
							aria-pressed={isSelected}
							aria-label={day.toLocaleDateString(getLocale(), { dateStyle: 'full' })}
							onclick={() => commit(day, time)}
							class="flex size-8 cursor-pointer items-center justify-center rounded-md text-sm disabled:cursor-not-allowed disabled:opacity-40 {isSelected
								? 'bg-primary font-semibold text-primary-foreground'
								: sameDay(new Date(), day)
									? 'text-foreground ring-1 ring-primary hover:bg-surface-muted'
									: 'text-foreground hover:bg-surface-muted'}"
						>
							{day.getDate()}
						</button>
					{:else}
						<span></span>
					{/if}
				{/each}
			</div>

			<div class="flex items-end justify-between gap-3 border-t border-border pt-3">
				<div class="grid gap-1.5">
					<span id="{id}-time" class="text-xs font-medium text-muted-foreground"
						>{m.datetime_picker_time()}</span
					>
					<div role="group" aria-labelledby="{id}-time" class="flex items-center gap-1">
						<input
							inputmode="numeric"
							maxlength="2"
							aria-label={m.datetime_picker_hours()}
							value={hours}
							disabled={!selected}
							onchange={(event) =>
								setTimePart('hours', event.currentTarget, parseInt(event.currentTarget.value, 10))}
							onkeydown={(event) => stepTimePart('hours', event)}
							class="w-11 rounded-lg border border-border bg-background px-2 py-1.5 text-center text-sm text-foreground tabular-nums focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-50"
						/>
						<span class="text-muted-foreground" aria-hidden="true">:</span>
						<input
							inputmode="numeric"
							maxlength="2"
							aria-label={m.datetime_picker_minutes()}
							value={minutes}
							disabled={!selected}
							onchange={(event) =>
								setTimePart(
									'minutes',
									event.currentTarget,
									parseInt(event.currentTarget.value, 10)
								)}
							onkeydown={(event) => stepTimePart('minutes', event)}
							class="w-11 rounded-lg border border-border bg-background px-2 py-1.5 text-center text-sm text-foreground tabular-nums focus:border-ring focus:ring-2 focus:ring-ring/40 focus:outline-none disabled:opacity-50"
						/>
					</div>
				</div>
				<button
					type="button"
					onclick={() => close()}
					class="cursor-pointer rounded-lg bg-primary px-4 py-1.5 text-sm font-semibold text-primary-foreground hover:bg-primary-hover"
				>
					{m.datetime_picker_done()}
				</button>
			</div>
		</div>
	{/if}
</div>
