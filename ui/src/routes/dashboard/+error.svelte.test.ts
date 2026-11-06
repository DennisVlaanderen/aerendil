import { beforeEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

// The page store is what a route error page reads: `$app/state` is a module in
// a browser test, so the component's import is replaced with this object and
// each case writes the status and message before it renders.
const { pageState } = vi.hoisted(() => ({
	pageState: { status: 500, error: { message: '' } } as {
		status: number;
		error: { message: string } | null;
	}
}));

vi.mock('$app/state', () => ({ page: pageState }));

describe('dashboard error page', () => {
	// The module cache is shared with the other browser tests in this project,
	// so the component is imported after the store has been set: a static import
	// would keep whichever `$app/state` the shared graph resolved first.
	beforeEach(() => {
		vi.resetModules();
		pageState.status = 500;
		pageState.error = { message: 'The dashboard could not be loaded.' };
	});

	test('a 403 reads as missing access instead of a crash', async () => {
		pageState.status = 403;
		pageState.error = { message: 'You do not have permission to view users.' };

		const { default: ErrorPage } = await import('./+error.svelte');
		const screen = await render(ErrorPage);

		await expect
			.element(screen.getByRole('heading', { name: "You don't have access to this page" }))
			.toBeVisible();
		await expect
			.element(screen.getByText('You do not have permission to view users.'))
			.toBeVisible();
		await expect
			.element(screen.getByText('Ask an administrator to add you to a group that has access.'))
			.toBeVisible();
		await expect.element(screen.getByRole('link', { name: 'Back to dashboard' })).toBeVisible();
		// The status label and the alert wording are what made it look like a fault.
		expect(screen.container.textContent).not.toContain('Error 403');
		expect(screen.container.textContent).not.toContain('Something went wrong');
	});

	test('any other status keeps the layout it had', async () => {
		const { default: ErrorPage } = await import('./+error.svelte');
		const screen = await render(ErrorPage);

		await expect.element(screen.getByText('Error 500')).toBeVisible();
		await expect
			.element(screen.getByRole('heading', { name: 'Something went wrong' }))
			.toBeVisible();
		await expect.element(screen.getByText('The dashboard could not be loaded.')).toBeVisible();
		await expect.element(screen.getByRole('link', { name: 'Back to dashboard' })).toBeVisible();
		expect(screen.container.textContent).not.toContain("You don't have access to this page");
	});
});
