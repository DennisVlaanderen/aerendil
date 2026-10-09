<script lang="ts">
	interface Props {
		title: string;
		description?: string;
		confirmLabel: string;
		cancelLabel: string;
		variant?: 'danger' | 'default';
		onconfirm: () => void;
		oncancel?: () => void;
	}

	let {
		title,
		description,
		confirmLabel,
		cancelLabel,
		variant = 'default',
		onconfirm,
		oncancel
	}: Props = $props();

	let dialogEl: HTMLDialogElement | undefined = $state();
	let confirmed = false;

	export function show() {
		confirmed = false;
		dialogEl?.showModal();
	}

	function requestConfirm() {
		confirmed = true;
		dialogEl?.close();
	}

	function requestCancel() {
		dialogEl?.close();
	}

	function handleBackdropClick(event: MouseEvent) {
		if (event.target === dialogEl) {
			requestCancel();
		}
	}

	// `close()` fires the native `close` event for every dismissal path --
	// the Confirm click above, the Cancel click, and the Escape key -- so
	// `confirmed` is the single source of truth for which one happened,
	// rather than wiring up separate handlers that could double-fire.
	function handleClose() {
		if (confirmed) {
			onconfirm();
		} else {
			oncancel?.();
		}
	}
</script>

<dialog
	bind:this={dialogEl}
	onclose={handleClose}
	onclick={handleBackdropClick}
	class="m-auto rounded-xl border border-border bg-surface p-0 backdrop:bg-ink-950/50"
>
	<div class="grid gap-4 p-6 sm:w-96">
		<div class="grid gap-1.5">
			<h2 class="text-lg font-semibold text-foreground">{title}</h2>
			{#if description}
				<p class="text-sm text-muted-foreground">{description}</p>
			{/if}
		</div>
		<div class="flex justify-end gap-3">
			<button
				type="button"
				class="cursor-pointer rounded-lg px-4 py-2 text-sm font-medium text-foreground hover:bg-surface-muted"
				onclick={requestCancel}
			>
				{cancelLabel}
			</button>
			<button
				type="button"
				class="cursor-pointer rounded-lg px-4 py-2 text-sm font-semibold {variant === 'danger'
					? 'bg-danger text-danger-foreground hover:opacity-90'
					: 'bg-primary text-primary-foreground hover:bg-primary-hover'}"
				onclick={requestConfirm}
			>
				{confirmLabel}
			</button>
		</div>
	</div>
</dialog>
