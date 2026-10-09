<script lang="ts">
	import './layout.css';

	let { children } = $props();
</script>

<svelte:head><link rel="icon" type="image/svg+xml" href="/favicon.svg" /></svelte:head>
{@render children()}
