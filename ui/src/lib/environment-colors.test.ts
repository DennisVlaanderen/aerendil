import { describe, expect, it } from 'vitest';
import { ENVIRONMENT_COLORS, resolveEnvironmentColor } from './environment-colors.ts';

describe('resolveEnvironmentColor', () => {
	it.each([
		['4', ENVIRONMENT_COLORS['4']],
		['#1a2b3c', '#1a2b3c'],
		['#abc', '#abc'],
		['', null],
		[undefined, null],
		['7', null],
		['#12345', null]
	])('resolves %s to %s', (input, expected) => {
		expect(resolveEnvironmentColor(input)).toBe(expected);
	});
});
