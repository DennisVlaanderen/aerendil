<script lang="ts">
	import type { Pathname } from '$app/types';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { localizeHref } from '$lib/paraglide/runtime';
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
	// Application credentials are treated as a user-like principal (they
	// resolve to the same *auth.User-shaped principal a human login does --
	// see backend/internal/auth/service.go's AuthenticateToken), so their
	// admin UI lives under User Management alongside Users/Groups rather
	// than under Application Settings.
	const canSeeApplicationCredentials = $derived(
		hasPermission({ isAdmin, permissions }, 'applicationCredentials:read')
	);
	const canSeeUserManagement = $derived(
		canSeeUsers || canSeeGroups || canSeeApplicationCredentials
	);
	const canSeeAuditLog = $derived(hasPermission({ isAdmin, permissions }, 'audits:read'));
	const canCreateFlags = $derived(hasPermission({ isAdmin, permissions }, 'flags:write'));
	const canSeeApplicationSettings = $derived(
		hasPermission({ isAdmin, permissions }, 'environments:read')
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
	<div class="flex items-center gap-2.5 border-b border-line-2 px-4.5 py-5 h-16">
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
			href={resolve(localizeHref('/dashboard') as Pathname)}
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
					isActive('/dashboard/groups') ||
					isActive('/dashboard/application-credentials')
						? 'bg-nav-active-bg text-nav-active'
						: 'text-nav-inactive'}"
				>
					<a
						class="flex flex-1 items-center gap-3 truncate px-2.5 py-2.25 text-[13.5px] font-medium no-underline"
						href={resolve(
							localizeHref(
								canSeeUsers
									? '/dashboard/users'
									: canSeeGroups
										? '/dashboard/groups'
										: '/dashboard/application-credentials'
							) as Pathname
						)}
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
								href={resolve(localizeHref('/dashboard/users') as Pathname)}
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
								href={resolve(localizeHref('/dashboard/groups') as Pathname)}
							>
								{m.nav_groups()}
							</a>
						{/if}
						{#if canSeeApplicationCredentials}
							<a
								class="flex items-center gap-2.5 truncate rounded-md px-2.5 py-1.75 text-[13px] font-medium no-underline hover:bg-line-3 {isActive(
									'/dashboard/application-credentials'
								)
									? 'bg-nav-active-bg text-nav-active'
									: 'text-nav-inactive'}"
								href={resolve(localizeHref('/dashboard/application-credentials') as Pathname)}
							>
								{m.nav_application_credentials()}
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
					)
						? 'bg-nav-active-bg text-nav-active'
						: 'text-nav-inactive'}"
				>
					<a
						class="flex flex-1 items-center gap-3 truncate px-2.5 py-2.25 text-[13.5px] font-medium no-underline"
						href={resolve(localizeHref('/dashboard/settings/environments') as Pathname)}
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
						<a
							class="flex items-center gap-2.5 truncate rounded-md px-2.5 py-1.75 text-[13px] font-medium no-underline hover:bg-line-3 {isActive(
								'/dashboard/settings/environments'
							)
								? 'bg-nav-active-bg text-nav-active'
								: 'text-nav-inactive'}"
							href={resolve(localizeHref('/dashboard/settings/environments') as Pathname)}
						>
							{m.nav_environments()}
						</a>
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
				href={resolve(localizeHref('/dashboard/audit-log') as Pathname)}
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
						href={resolve(localizeHref('/dashboard/flags/new') as Pathname)}
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
					href={resolve(localizeHref(`/dashboard/flags/${flag.key}`) as Pathname)}
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
		class="flex cursor-pointer items-center gap-2 border-t border-line-2 px-4 py-3 text-ink-muted h-10"
		onclick={() => (collapsed = !collapsed)}
		aria-label={collapsed ? m.sidebar_expand() : m.sidebar_collapse()}
	>
		<span
			class="{collapsed
				? 'icon-[lucide--panel-left-open]'
				: 'icon-[lucide--panel-left-close]'} size-3.5 shrink-0"
			aria-hidden="true"
		></span>
		{#if !collapsed}<span class="text-xs overflow-hidden h-4">{m.sidebar_collapse()}</span>{/if}
	</button>
</aside>
