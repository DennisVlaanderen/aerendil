import { describe, expect, it } from 'vitest';
import {
	evenWeights,
	parseValues,
	parseVariationValue,
	segmentUsage,
	totalWeight,
	uniqueId,
	valueKind,
	variationText,
	type FlagDefinition
} from './flags';

describe('parseValues', () => {
	it('parses one value per line by kind, dropping blank lines', () => {
		expect(parseValues('NL\n\nBE \n', 'string')).toEqual(['NL', 'BE ']);
		expect(parseValues('5\n 7.5', 'number')).toEqual([5, 7.5]);
		expect(parseValues('true\nfalse', 'boolean')).toEqual([true, false]);
	});
});

describe('valueKind', () => {
	it('follows the values for in, and the op otherwise', () => {
		expect(valueKind({ op: 'in', values: [5] })).toBe('number');
		expect(valueKind({ op: 'in', values: [] })).toBe('string');
		expect(valueKind({ op: 'gt', values: [] })).toBe('number');
	});
});

describe('variation values', () => {
	it('round-trips each flag type through text', () => {
		expect(parseVariationValue(variationText(42, 'integer'), 'integer')).toBe(42);
		expect(parseVariationValue(variationText({ a: 1 }, 'json'), 'json')).toEqual({ a: 1 });
		expect(parseVariationValue(variationText('v2', 'string'), 'string')).toBe('v2');
		expect(parseVariationValue(variationText(false, 'boolean'), 'boolean')).toBe(false);
	});

	it('rejects a bad integer or JSON value', () => {
		expect(() => parseVariationValue('1.5', 'integer')).toThrow();
		expect(() => parseVariationValue('', 'integer')).toThrow();
		expect(() => parseVariationValue('{', 'json')).toThrow();
	});

	it('accepts a number from a number input', () => {
		expect(parseVariationValue(7, 'integer')).toBe(7);
	});
});

describe('evenWeights', () => {
	it('always sums to 100000', () => {
		for (const n of [1, 2, 3, 7]) {
			const ids = Array.from({ length: n }, (_, i) => `v${i}`);
			expect(totalWeight(evenWeights(ids))).toBe(100000);
		}
		expect(evenWeights(['a', 'b', 'c'])).toEqual([
			{ variation: 'a', weight: 33334 },
			{ variation: 'b', weight: 33333 },
			{ variation: 'c', weight: 33333 }
		]);
	});
});

describe('uniqueId', () => {
	it('skips ids already taken', () => {
		expect(uniqueId('rule', [])).toBe('rule-1');
		expect(uniqueId('rule', ['rule-2', 'x'])).toBe('rule-3');
	});
});

describe('segmentUsage', () => {
	it('lists the flags whose rules reference the segment', () => {
		const flag = (key: string, segment: string) =>
			({
				key,
				rules: [{ id: 'r', conditions: [{ op: 'inSegment', values: [segment] }], serve: {} }]
			}) as unknown as FlagDefinition;
		expect(segmentUsage([flag('a', 'beta'), flag('b', 'staff')], 'beta')).toEqual(['a']);
	});
});
