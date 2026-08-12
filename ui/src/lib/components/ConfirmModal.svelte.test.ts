import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ConfirmModal from './ConfirmModal.svelte';

const baseProps = {
	title: 'Delete item?',
	description: 'This cannot be undone.',
	confirmLabel: 'Delete',
	cancelLabel: 'Cancel'
};

describe('ConfirmModal', () => {
	test('is not visible until show() is called', async () => {
		const screen = await render(ConfirmModal, { ...baseProps, onconfirm: vi.fn() });

		await expect.element(screen.getByText('Delete item?')).not.toBeVisible();
	});

	test('shows the dialog when show() is called via the component instance', async () => {
		const screen = await render(ConfirmModal, { ...baseProps, onconfirm: vi.fn() });

		screen.component.show();

		await expect.element(screen.getByText('Delete item?')).toBeVisible();
		await expect.element(screen.getByText('This cannot be undone.')).toBeVisible();
	});

	test('calls onconfirm and closes when the confirm button is clicked', async () => {
		const onconfirm = vi.fn();
		const screen = await render(ConfirmModal, { ...baseProps, onconfirm });
		screen.component.show();

		await screen.getByRole('button', { name: 'Delete' }).click();

		expect(onconfirm).toHaveBeenCalledOnce();
		await expect.element(screen.getByText('Delete item?')).not.toBeVisible();
	});

	test('calls oncancel and closes when the cancel button is clicked, without confirming', async () => {
		const onconfirm = vi.fn();
		const oncancel = vi.fn();
		const screen = await render(ConfirmModal, { ...baseProps, onconfirm, oncancel });
		screen.component.show();

		await screen.getByRole('button', { name: 'Cancel' }).click();

		expect(oncancel).toHaveBeenCalledOnce();
		expect(onconfirm).not.toHaveBeenCalled();
		await expect.element(screen.getByText('Delete item?')).not.toBeVisible();
	});
});
