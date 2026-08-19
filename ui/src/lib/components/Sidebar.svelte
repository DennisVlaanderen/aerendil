<script lang="ts">
	import { page } from '$app/state';
	import { localizedResolve } from '$lib/localizedResolve';
	import { m } from '$lib/paraglide/messages.js';
	import LocaleSwitcher from './LocaleSwitcher.svelte';
	import type { FlagSummary } from '$lib/server/flags';
	import { hasPermission } from '$lib/permissions';
	import { getInitials } from '$lib/initials';

	let {
		flags,
		username,
		isAdmin,
		permissions
	}: { flags: FlagSummary[]; username: string; isAdmin: boolean; permissions: string[] } = $props();

	let collapsed = $state(false);
	let userManagementOpen = $state(false);
	let userManagementContainer: HTMLDivElement | undefined = $state();
	let applicationSettingsOpen = $state(false);
	let applicationSettingsContainer: HTMLDivElement | undefined = $state();

	const canSeeUsers = $derived(hasPermission({ isAdmin, permissions }, 'users:read'));
	const canSeeGroups = $derived(hasPermission({ isAdmin, permissions }, 'groups:read'));
	const canSeeUserManagement = $derived(canSeeUsers || canSeeGroups);
	const canSeeAuditLog = $derived(hasPermission({ isAdmin, permissions }, 'audits:read'));
	const canCreateFlags = $derived(hasPermission({ isAdmin, permissions }, 'flags:write'));
	const canSeeEnvironments = $derived(hasPermission({ isAdmin, permissions }, 'environments:read'));
	// Application credentials are an "Application Settings" concept (they
	// configure how an application authenticates to Aerendil, the same way
	// environments configure how flags are scoped), not a User Management
	// one -- grouped here alongside Environments rather than next to
	// Users/Groups.
	const canSeeApplicationCredentials = $derived(
		hasPermission({ isAdmin, permissions }, 'applicationCredentials:read')
	);
	const canSeeApplicationSettings = $derived(canSeeEnvironments || canSeeApplicationCredentials);

	// firstVisiblePath picks a dropdown's default link target: the first
	// entry whose guard is true, falling back to the first entry so the
	// link always resolves to something even if called before any guard is
	// known -- adding a new item to a dropdown is "add one more [guard,
	// path] pair" instead of growing a nested ternary.
	function firstVisiblePath(...entries: [boolean, string][]): string {
		return entries.find(([visible]) => visible)?.[1] ?? entries[0][1];
	}
	const userManagementDefaultPath = $derived(
		firstVisiblePath([canSeeUsers, '/dashboard/users'], [canSeeGroups, '/dashboard/groups'])
	);
	const applicationSettingsDefaultPath = $derived(
		firstVisiblePath(
			[canSeeEnvironments, '/dashboard/settings/environments'],
			[canSeeApplicationCredentials, '/dashboard/application-credentials']
		)
	);

	function isActive(pathname: string) {
		return page.url.pathname === pathname;
	}

	function handleClickOutsideDropdowns(event: MouseEvent) {
		if (
			userManagementOpen &&
			userManagementContainer &&
			!userManagementContainer.contains(event.target as Node)
		) {
			userManagementOpen = false;
		}
		if (
			applicationSettingsOpen &&
			applicationSettingsContainer &&
			!applicationSettingsContainer.contains(event.target as Node)
		) {
			applicationSettingsOpen = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			userManagementOpen = false;
			applicationSettingsOpen = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutsideDropdowns} onkeydown={handleKeydown} />

<aside
	class="flex h-full shrink-0 flex-col border-r border-line-2 bg-sidebar transition-[width] duration-200 {collapsed
		? 'w-18'
		: 'w-62.5'}"
>
	<div class="flex h-16 items-center gap-2.5 border-b border-line-2 px-4.5 py-5">
		<img src="/aerendil-logo.svg" width="26" height="26" class="shrink-0" alt="Aerendil Logo" />
		{#if !collapsed}
			<span class="truncate text-base font-semibold tracking-[0.3px] text-ink">Aerendil</span>
		{/if}
	</div>

	<nav class="flex flex-1 flex-col gap-1 overflow-y-auto p-2.5">
		<a
			class="flex items-center gap-3 truncate rounded-lg px-2.5 py-2.25 text-[13.5px] font-medium no-underline hover:bg-line-3 {isActive(
				'/dashboard'
			)
				? 'bg-nav-active-bg text-nav-active'
				: 'text-nav-inactive'}"
			href={localizedResolve('/dashboard')}
		>
			<span class="flex w-4.5 shrink-0 justify-center" aria-hidden="true">
				<span class="icon-[lucide--layout-dashboard] size-4.5"></span>
			</span>
			{#if !collapsed}<span>{m.nav_dashboard()}</span>{/if}
		</a>

		{#if canSeeUserManagement}
			<div bind:this={userManagementContainer}>
				<div
					class="flex items-center gap-1 rounded-lg hover:bg-line-3 {isActive('/dashboard/users') ||
					isActive('/dashboard/groups')
						? 'bg-nav-active-bg text-nav-active'
						: 'text-nav-inactive'}"
				>
					<a
						class="flex flex-1 items-center gap-3 truncate px-2.5 py-2.25 text-[13.5px] font-medium no-underline"
						href={localizedResolve(userManagementDefaultPath)}
					>
						<span class="flex w-4.5 shrink-0 justify-center" aria-hidden="true">
							<span class="icon-[lucide--users] size-4.5"></span>
						</span>
						{#if !collapsed}<span>{m.nav_user_management()}</span>{/if}
					</a>
					{#if !collapsed}
						<button
							type="button"
							class="flex shrink-0 cursor-pointer items-center justify-center px-2.5 py-2.25"
							aria-haspopup="true"
							aria-expanded={userManagementOpen}
							aria-label={m.nav_user_management_toggle()}
							onclick={() => (userManagementOpen = !userManagementOpen)}
						>
							<span
								class="icon-[lucide--chevron-down] size-3.5 transition-transform duration-150 {userManagementOpen
									? 'rotate-180'
									: ''}"
								aria-hidden="true"
							></span>
						</button>
					{/if}
				</div>

				{#if userManagementOpen && !collapsed}
					<div class="mt-0.5 ml-7.5 flex flex-col gap-0.5 border-l border-line-3 pl-2.5">
						{#if canSeeUsers}
							<a
								class="flex items-center gap-2.5 truncate rounded-md px-2.5 py-1.75 text-[13px] font-medium no-underline hover:bg-line-3 {isActive(
									'/dashboard/users'
								)
									? 'bg-nav-active-bg text-nav-active'
									: 'text-nav-inactive'}"
								href={localizedResolve('/dashboard/users')}
							>
								{m.nav_users()}
							</a>
						{/if}
						{#if canSeeGroups}
							<a
								class="flex items-center gap-2.5 truncate rounded-md px-2.5 py-1.75 text-[13px] font-medium no-underline hover:bg-line-3 {isActive(
									'/dashboard/groups'
								)
									? 'bg-nav-active-bg text-nav-active'
									: 'text-nav-inactive'}"
								href={localizedResolve('/dashboard/groups')}
							>
								{m.nav_groups()}
							</a>
						{/if}
					</div>
				{/if}
			</div>
		{/if}

		{#if canSeeApplicationSettings}
			<div bind:this={applicationSettingsContainer}>
				<div
					class="flex items-center gap-1 rounded-lg hover:bg-line-3 {isActive(
						'/dashboard/settings/environments'
					) || isActive('/dashboard/application-credentials')
						? 'bg-nav-active-bg text-nav-active'
						: 'text-nav-inactive'}"
				>
					<a
						class="flex flex-1 items-center gap-3 truncate px-2.5 py-2.25 text-[13.5px] font-medium no-underline"
						href={localizedResolve(applicationSettingsDefaultPath)}
					>
						<span class="flex w-4.5 shrink-0 justify-center" aria-hidden="true">
							<span class="icon-[lucide--settings] size-4.5"></span>
						</span>
						{#if !collapsed}<span>{m.nav_application_settings()}</span>{/if}
					</a>
					{#if !collapsed}
						<button
							type="button"
							class="flex shrink-0 cursor-pointer items-center justify-center px-2.5 py-2.25"
							aria-haspopup="true"
							aria-expanded={applicationSettingsOpen}
							aria-label={m.nav_application_settings_toggle()}
							onclick={() => (applicationSettingsOpen = !applicationSettingsOpen)}
						>
							<span
								class="icon-[lucide--chevron-down] size-3.5 transition-transform duration-150 {applicationSettingsOpen
									? 'rotate-180'
									: ''}"
								aria-hidden="true"
							></span>
						</button>
					{/if}
				</div>

				{#if applicationSettingsOpen && !collapsed}
					<div class="mt-0.5 ml-7.5 flex flex-col gap-0.5 border-l border-line-3 pl-2.5">
						{#if canSeeEnvironments}
							<a
								class="flex items-center gap-2.5 truncate rounded-md px-2.5 py-1.75 text-[13px] font-medium no-underline hover:bg-line-3 {isActive(
									'/dashboard/settings/environments'
								)
									? 'bg-nav-active-bg text-nav-active'
									: 'text-nav-inactive'}"
								href={localizedResolve('/dashboard/settings/environments')}
							>
								{m.nav_environments()}
							</a>
						{/if}
						{#if canSeeApplicationCredentials}
							<a
								class="flex items-center gap-2.5 truncate rounded-md px-2.5 py-1.75 text-[13px] font-medium no-underline hover:bg-line-3 {isActive(
									'/dashboard/application-credentials'
								)
									? 'bg-nav-active-bg text-nav-active'
									: 'text-nav-inactive'}"
								href={localizedResolve('/dashboard/application-credentials')}
							>
								{m.nav_application_credentials()}
							</a>
						{/if}
					</div>
				{/if}
			</div>
		{/if}

		{#if canSeeAuditLog}
			<a
				class="flex items-center gap-3 truncate rounded-lg px-2.5 py-2.25 text-[13.5px] font-medium no-underline hover:bg-line-3 {isActive(
					'/dashboard/audit-log'
				)
					? 'bg-nav-active-bg text-nav-active'
					: 'text-nav-inactive'}"
				href={localizedResolve('/dashboard/audit-log')}
			>
				<span class="flex w-4.5 shrink-0 justify-center" aria-hidden="true">
					<span class="icon-[lucide--history] size-4.5"></span>
				</span>
				{#if !collapsed}<span>{m.nav_audit_log()}</span>{/if}
			</a>
		{/if}

		{#if !collapsed}
			<div class="mt-3 mb-0.5 flex items-center justify-between gap-1.5 px-2.5">
				<p
					class="flex items-center gap-1.5 text-[11px] font-semibold tracking-wide text-nav-inactive uppercase"
				>
					<span class="icon-[lucide--flag] size-3.5" aria-hidden="true"></span>
					{m.nav_flags()}
				</p>
				{#if canCreateFlags}
					<a
						class="flex size-4.5 shrink-0 items-center justify-center rounded text-nav-inactive no-underline hover:bg-line-3 hover:text-nav-active"
						href={localizedResolve('/dashboard/flags/new')}
						aria-label={m.nav_new_flag()}
					>
						<span class="icon-[lucide--plus] size-3.5" aria-hidden="true"></span>
					</a>
				{/if}
			</div>
		{/if}

		{#if flags.length === 0 && !collapsed}
			<p class="px-2.5 text-sm text-ink-muted">{m.nav_no_flags()}</p>
		{:else}
			{#each flags as flag (flag.key)}
				<a
					class="flex items-center gap-3 truncate rounded-lg px-2.5 py-2.25 text-[13.5px] font-medium no-underline hover:bg-line-3 {isActive(
						`/dashboard/flags/${flag.key}`
					)
						? 'bg-nav-active-bg text-nav-active'
						: 'text-nav-inactive'}"
					href={localizedResolve(`/dashboard/flags/${flag.key}`)}
				>
					<span class="flex w-4.5 shrink-0 justify-center" aria-hidden="true">
						{#if flag.enabled}
							<span class="icon-[lucide--toggle-right] size-4.5 text-success"></span>
						{:else}
							<span class="icon-[lucide--toggle-left] size-4.5 text-ink-muted"></span>
						{/if}
					</span>
					{#if !collapsed}<span>{flag.key}</span>{/if}
				</a>
			{/each}
		{/if}
	</nav>

	<div class="flex flex-col gap-0.5 border-t border-line-2 p-2.5">
		<LocaleSwitcher compact={collapsed} />
		<form method="POST" action="/logout">
			<button
				type="submit"
				class="flex w-full cursor-pointer items-center gap-3 truncate rounded-lg px-2.5 py-2.25 text-[13.5px] font-medium text-error hover:bg-line-3"
			>
				<span class="flex w-4.5 shrink-0 justify-center" aria-hidden="true">
					<span class="icon-[lucide--log-out] size-4.5"></span>
				</span>
				{#if !collapsed}<span>{m.logout_button()}</span>{/if}
			</button>
		</form>
	</div>

	<div class="flex items-center gap-2.5 border-t border-line-2 px-4 py-3.5">
		<div
			class="flex size-6.5 shrink-0 items-center justify-center rounded-full bg-avatar text-[11px] font-semibold text-cream"
		>
			{getInitials(username)}
		</div>
		{#if !collapsed}
			<div class="min-w-0">
				<div class="truncate text-[12.5px] font-medium text-ink">{username}</div>
				<div class="truncate text-[11px] text-ink-muted">
					{isAdmin ? m.sidebar_role_admin() : m.sidebar_role_member()}
				</div>
			</div>
		{/if}
	</div>

	<button
		type="button"
		class="flex h-10 cursor-pointer items-center gap-2 border-t border-line-2 px-4 py-3 text-ink-muted"
		onclick={() => (collapsed = !collapsed)}
		aria-label={collapsed ? m.sidebar_expand() : m.sidebar_collapse()}
	>
		<span
			class="{collapsed
				? 'icon-[lucide--panel-left-open]'
				: 'icon-[lucide--panel-left-close]'} size-3.5 shrink-0"
			aria-hidden="true"
		></span>
		{#if !collapsed}<span class="h-4 overflow-hidden text-xs">{m.sidebar_collapse()}</span>{/if}
	</button>
</aside>
