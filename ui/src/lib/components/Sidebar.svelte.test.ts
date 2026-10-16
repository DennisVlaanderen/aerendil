import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import Sidebar from './Sidebar.svelte';

const baseProps = {
	flags: [],
	username: 'someone'
};

describe('Sidebar user management visibility', () => {
	test('hides the User Management section for a user with no relevant permissions', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['flags:read']
		});

		await expect.element(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument();
		await expect.element(screen.getByText('User Management')).not.toBeInTheDocument();
	});

	test('shows the User Management section when the user has users:read', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['users:read']
		});

		await expect.element(screen.getByText('User Management')).toBeInTheDocument();
	});

	test('shows the User Management section when the user has groups:read', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['groups:read']
		});

		await expect.element(screen.getByText('User Management')).toBeInTheDocument();
	});

	test('shows the User Management section for an admin with no explicit permissions', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: true,
			permissions: []
		});

		await expect.element(screen.getByText('User Management')).toBeInTheDocument();
	});

	test('expands to show Users and Groups sub-links when clicked', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: true,
			permissions: []
		});

		await expect.element(screen.getByRole('link', { name: 'Users' })).not.toBeInTheDocument();

		await screen.getByRole('button', { name: 'Toggle User Management submenu' }).click();

		await expect.element(screen.getByRole('link', { name: 'Users' })).toBeInTheDocument();
		await expect.element(screen.getByRole('link', { name: 'Groups' })).toBeInTheDocument();
	});

	test('only shows the sub-link matching a partial permission grant', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['groups:read']
		});

		await screen.getByRole('button', { name: 'Toggle User Management submenu' }).click();

		await expect.element(screen.getByRole('link', { name: 'Groups' })).toBeInTheDocument();
		await expect.element(screen.getByRole('link', { name: 'Users' })).not.toBeInTheDocument();
	});
});

describe('Sidebar application settings visibility', () => {
	test('hides the Application Settings section for a user with no relevant permissions', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['flags:read']
		});

		await expect.element(screen.getByText('Application Settings')).not.toBeInTheDocument();
	});

	test('shows the Application Settings section when the user has environments:read', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['environments:read']
		});

		await expect.element(screen.getByText('Application Settings')).toBeInTheDocument();
	});

	test('shows the Application Settings section for an admin with no explicit permissions', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: true,
			permissions: []
		});

		await expect.element(screen.getByText('Application Settings')).toBeInTheDocument();
	});

	test('expands to show the Environments sub-link when clicked', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: true,
			permissions: []
		});

		await expect
			.element(screen.getByRole('link', { name: 'Environments' }))
			.not.toBeInTheDocument();

		await screen.getByRole('button', { name: 'Toggle Application Settings submenu' }).click();

		await expect.element(screen.getByRole('link', { name: 'Environments' })).toBeInTheDocument();
	});

	test('shows the Application Settings section when the user only has applicationCredentials:read', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['applicationCredentials:read']
		});

		await expect.element(screen.getByText('Application Settings')).toBeInTheDocument();
	});

	test('only shows the Applications sub-link matching a partial permission grant', async () => {
		const screen = await render(Sidebar, {
			...baseProps,
			isAdmin: false,
			permissions: ['applicationCredentials:read']
		});

		await screen.getByRole('button', { name: 'Toggle Application Settings submenu' }).click();

		await expect.element(screen.getByRole('link', { name: 'Applications' })).toBeInTheDocument();
		await expect
			.element(screen.getByRole('link', { name: 'Environments' }))
			.not.toBeInTheDocument();
	});
});

describe('Sidebar profile menu', () => {
	const props = { ...baseProps, isAdmin: false, permissions: [] };

	test('opens a menu with language, theme and logout', async () => {
		const screen = await render(Sidebar, props);

		await screen.getByRole('button', { name: 'Account menu' }).click();

		await expect
			.element(screen.getByRole('menuitemradio', { name: 'English' }))
			.toBeInTheDocument();
		await expect.element(screen.getByRole('menuitemradio', { name: 'Light' })).toBeInTheDocument();
		await expect.element(screen.getByRole('menuitemradio', { name: 'Dark' })).toBeInTheDocument();
		await expect.element(screen.getByRole('menuitemradio', { name: 'System' })).toBeInTheDocument();
		await expect.element(screen.getByRole('menuitem', { name: 'Log out' })).toBeInTheDocument();
	});

	test('sets the theme to light, dark or system', async () => {
		const screen = await render(Sidebar, props);
		const trigger = screen.getByRole('button', { name: 'Account menu' });

		await trigger.click();
		await screen.getByRole('menuitemradio', { name: 'Dark' }).click();
		expect(localStorage.getItem('aerendil-theme')).toBe('dark');

		await trigger.click();
		await screen.getByRole('menuitemradio', { name: 'System' }).click();
		expect(localStorage.getItem('aerendil-theme')).toBeNull();

		await trigger.click();
		await expect
			.element(screen.getByRole('menuitemradio', { name: 'System' }))
			.toHaveAttribute('aria-checked', 'true');
	});

	test('closes on Escape and returns focus to the trigger', async () => {
		const screen = await render(Sidebar, props);
		const trigger = screen.getByRole('button', { name: 'Account menu' });

		await trigger.click();
		await expect.element(screen.getByRole('menu')).toBeInTheDocument();
		await userEvent.keyboard('{Escape}');

		await expect.element(screen.getByRole('menu')).not.toBeInTheDocument();
		await expect.element(trigger).toHaveFocus();
	});

	test('still opens when the sidebar is collapsed', async () => {
		const screen = await render(Sidebar, props);

		await screen.getByRole('button', { name: 'Collapse sidebar' }).click();
		await screen.getByRole('button', { name: 'Account menu' }).click();

		await expect.element(screen.getByRole('menu')).toBeInTheDocument();
	});
});
