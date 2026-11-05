import { describe, expect, test } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import EnvironmentColorPicker from './EnvironmentColorPicker.svelte';

describe('EnvironmentColorPicker', () => {
	test('survives a form reset and submits an empty value', async () => {
		const form = document.createElement('form');
		document.body.append(form);

		const component = mount(EnvironmentColorPicker, {
			target: form,
			props: { name: 'color', label: 'Color', value: '4' }
		});

		expect(() => form.reset()).not.toThrow();
		await tick();
		expect(form.querySelector<HTMLInputElement>('input[type="hidden"]')?.value).toBe('');

		unmount(component);
		form.remove();
	});
});
