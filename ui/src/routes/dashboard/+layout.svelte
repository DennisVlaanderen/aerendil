<script lang="ts">
	import { page } from '$app/state';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import { toast } from '$lib/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	$effect(() => {
		if (page.data.flashReason === 'route-not-found') {
			toast.show(m.toast_route_not_found());
		}
	});
</script>

<div class="flex h-screen overflow-hidden bg-page">
	<Sidebar
		flags={data.flags}
		username={data.username}
		isAdmin={data.isAdmin}
		permissions={data.permissions}
	/>
	<div class="flex min-w-0 flex-1 flex-col">
		<Header
			username={data.username}
			environments={data.environments}
			selectedEnvironmentId={data.selectedEnvironmentId}
		/>
		<main class="min-w-0 flex-1 overflow-y-auto">
			{@render children()}
		</main>
	</div>
</div>

<Toast />
