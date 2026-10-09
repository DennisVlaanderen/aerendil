import { describe, expect, test } from 'vitest';
import { userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import DateTimePicker from './DateTimePicker.svelte';

function hiddenInput(container: HTMLElement): HTMLInputElement {
	return container.querySelector('input[type="hidden"]') as HTMLInputElement;
}

const utc = (date: Date) => date.toISOString().replace(/\.\d{3}Z$/, 'Z');

describe('DateTimePicker', () => {
	test('submits the bound UTC value unchanged under its name', async () => {
		const value = '2026-10-09T11:30:00Z';
		const screen = await render(DateTimePicker, { name: 'from', label: 'From', value });

		expect(hiddenInput(screen.container)).toHaveProperty('name', 'from');
		expect(hiddenInput(screen.container).value).toBe(value);
		await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
	});

	test('picking a day keeps the current local time and submits it as UTC', async () => {
		const value = '2026-10-09T11:30:00Z';
		const screen = await render(DateTimePicker, { name: 'from', label: 'From', value });

		await screen.getByRole('button', { name: /From/ }).click();
		await expect.element(screen.getByRole('dialog')).toBeVisible();

		const start = new Date(value);
		const picked = new Date(
			start.getFullYear(),
			start.getMonth(),
			20,
			start.getHours(),
			start.getMinutes()
		);
		await screen
			.getByRole('button', { name: picked.toLocaleDateString('en', { dateStyle: 'full' }) })
			.click();

		expect(hiddenInput(screen.container).value).toBe(utc(picked));
	});

	test('days past max are disabled', async () => {
		const screen = await render(DateTimePicker, {
			name: 'from',
			label: 'From',
			value: '2026-10-09T11:30:00Z',
			max: '2026-10-12T11:30:00Z'
		});

		await screen.getByRole('button', { name: /From/ }).click();
		const day = (d: number) =>
			screen.getByRole('button', {
				name: new Date(2026, 9, d).toLocaleDateString('en', { dateStyle: 'full' })
			});
		await expect.element(day(11)).toBeEnabled();
		await expect.element(day(20)).toBeDisabled();
	});

	test('the clear button empties the submitted value', async () => {
		const screen = await render(DateTimePicker, {
			name: 'to',
			label: 'To',
			value: '2026-10-09T11:30:00Z'
		});

		await screen.getByRole('button', { name: 'Clear' }).click();

		expect(hiddenInput(screen.container).value).toBe('');
	});

	test('Escape closes the calendar', async () => {
		const screen = await render(DateTimePicker, { name: 'to', label: 'To' });

		await screen.getByRole('button', { name: /To/ }).click();
		await expect.element(screen.getByRole('dialog')).toBeVisible();
		(document.activeElement as HTMLElement).dispatchEvent(
			new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
		);

		await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
	});

	test('typed hours are clamped and arrow keys wrap minutes', async () => {
		const start = new Date(2026, 9, 9, 11, 59);
		const screen = await render(DateTimePicker, { name: 'from', label: 'From', value: utc(start) });
		await screen.getByRole('button', { name: /From/ }).click();

		await screen.getByRole('textbox', { name: 'Hours' }).fill('27');
		// Moving focus on fires change for the hours field.
		await screen.getByRole('textbox', { name: 'Minutes' }).click();
		expect(hiddenInput(screen.container).value).toBe(utc(new Date(2026, 9, 9, 23, 59)));

		await userEvent.keyboard('{ArrowUp}');
		expect(hiddenInput(screen.container).value).toBe(utc(new Date(2026, 9, 9, 23, 0)));
	});
});
