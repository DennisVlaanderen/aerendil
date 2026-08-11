<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';

	onMount(() => {
		void goto(resolve('/login'));
	});
</script>

<div class="redirecting">Redirecting to the sign-in page…</div>

<style>
	.redirecting {
		display: grid;
		place-items: center;
		min-height: 100vh;
		font-size: 1.1rem;
		font-weight: 600;
		color: var(--color-ink);
	}
</style>
