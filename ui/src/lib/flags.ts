// docs/fqdp.md §6 flag and segment definitions as the backend's HTTP API
// sends and accepts them, plus the pure helpers the editors share. Client-safe
// (no lib/server imports) so components can use it.

export type FlagType = 'boolean' | 'integer' | 'string' | 'json';
export const FLAG_TYPES: FlagType[] = ['boolean', 'integer', 'string', 'json'];

export type ConditionValue = string | number | boolean;

export interface Condition {
	attribute?: string;
	op: string;
	values: ConditionValue[];
	negate?: boolean;
}

export interface RolloutWeight {
	variation: string;
	weight: number;
}

export interface Serve {
	variation?: string;
	rollout?: { bucketBy?: string; weights: RolloutWeight[] };
}

export interface Rule {
	id: string;
	description?: string;
	match?: 'all' | 'any';
	conditions: Condition[];
	serve: Serve;
}

export interface Variation {
	id: string;
	value: unknown;
}

export interface FlagDefinition {
	environmentId: string;
	key: string;
	type: FlagType;
	enabled: boolean;
	revision: number;
	variations: Variation[];
	offVariation: string;
	rules: Rule[];
	fallthrough: Serve;
}

export interface SegmentRule {
	id: string;
	match?: 'all' | 'any';
	conditions: Condition[];
}

export interface Segment {
	environmentId: string;
	key: string;
	name: string;
	revision: number;
	included: string[];
	excluded: string[];
	rules: SegmentRule[];
}

export interface EvaluationResult {
	value?: unknown;
	variationId?: string;
	reason: string;
	ruleId?: string;
	errorKind?: string;
}

// §6.3 operators; inSegment takes segment keys and no attribute.
export const OPS = [
	'in',
	'contains',
	'startsWith',
	'endsWith',
	'lt',
	'lte',
	'gt',
	'gte',
	'inSegment'
] as const;

export type ValueKind = 'string' | 'number' | 'boolean';

// valueKinds lists the §6.3 value kinds op accepts; only `in` has a choice.
export function valueKinds(op: string): ValueKind[] {
	if (op === 'in') return ['string', 'number', 'boolean'];
	if (['lt', 'lte', 'gt', 'gte'].includes(op)) return ['number'];
	return ['string'];
}

export function valueKind(condition: Condition): ValueKind {
	const kind = typeof condition.values[0];
	const kinds = valueKinds(condition.op);
	return kinds.includes(kind as ValueKind) ? (kind as ValueKind) : kinds[0];
}

// parseValues turns the editor's one-value-per-line text into condition
// values of kind; blank lines are dropped. Unparseable numbers stay NaN so
// the backend rejects them instead of the UI silently dropping them.
export function parseValues(text: string, kind: ValueKind): ConditionValue[] {
	const lines = text
		.split('\n')
		.map((line) => (kind === 'string' ? line : line.trim()))
		.filter((line) => line.trim() !== '');
	if (kind === 'number') return lines.map(Number);
	if (kind === 'boolean') return lines.map((line) => line === 'true');
	return lines;
}

export function formatValues(values: ConditionValue[]): string {
	return values.map(String).join('\n');
}

// Variation values are edited as text; variationText/parseVariationValue
// convert to and from a flag type's JSON value, throwing on a bad value.
export function variationText(value: unknown, type: FlagType): string {
	if (type === 'string') return typeof value === 'string' ? value : '';
	return value === undefined ? '' : JSON.stringify(value, null, type === 'json' ? 2 : 0);
}

// text may arrive as a number: bind:value on a number input yields one.
export function parseVariationValue(text: string | number | null, type: FlagType): unknown {
	text = String(text ?? '');
	if (type === 'string') return text;
	if (type === 'integer') {
		const n = Number(text.trim());
		if (text.trim() === '' || !Number.isSafeInteger(n)) throw new Error('not an integer');
		return n;
	}
	if (type === 'boolean') return text === 'true';
	return JSON.parse(text);
}

// A boolean flag's variations are fixed by §6.1.
export const BOOLEAN_VARIATIONS: Variation[] = [
	{ id: 'true', value: true },
	{ id: 'false', value: false }
];

// Rollout weights are on a 100000 scale (§6.5); the editor shows percent
// with three decimals, so 1 bucket = 0.001%.
export const ROLLOUT_SCALE = 100000;

export function evenWeights(variationIds: string[]): RolloutWeight[] {
	const share = Math.floor(ROLLOUT_SCALE / variationIds.length);
	return variationIds.map((variation, i) => ({
		variation,
		weight: i === 0 ? ROLLOUT_SCALE - share * (variationIds.length - 1) : share
	}));
}

export function totalWeight(weights: RolloutWeight[]): number {
	return weights.reduce((sum, w) => sum + w.weight, 0);
}

// uniqueId returns prefix-N for the lowest N not already in ids.
export function uniqueId(prefix: string, ids: string[]): string {
	let n = ids.length + 1;
	while (ids.includes(`${prefix}-${n}`)) n++;
	return `${prefix}-${n}`;
}

// segmentUsage lists the flags whose rules reference segmentKey, i.e. why a
// segment delete would be refused (fqdp.md §6.2).
export function segmentUsage(flags: FlagDefinition[], segmentKey: string): string[] {
	return flags
		.filter((flag) =>
			(flag.rules ?? []).some((rule) =>
				rule.conditions.some((c) => c.op === 'inSegment' && c.values.includes(segmentKey))
			)
		)
		.map((flag) => flag.key);
}
