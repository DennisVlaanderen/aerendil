import { describe, expect, it } from 'vitest';
import { overwriteGetLocale } from './paraglide/runtime';
import { errorMessages, resolveErrorMessage } from './errors';
import en from '../../messages/en.json';
import nlNl from '../../messages/nl-nl.json';

// Paraglide's message functions call getLocale() internally, which needs a
// request/URL context (the project's locale strategy is 'url') that this
// plain-node test suite doesn't have -- stub it to 'en' so invoking m.xxx()
// below doesn't throw "No locale found".
overwriteGetLocale(() => 'en');

// errorMessages' values are the actual m.xxx paraglide message functions --
// their .name is the underlying message key (paraglide compiles each to a
// named function), so this can check the real messages/*.json files
// directly instead of trusting the generated (gitignored, rebuilt-on-build)
// paraglide output to still be in sync.
describe('errorMessages', () => {
	it('maps every code to a message key present in both en.json and nl-nl.json', () => {
		for (const [code, fn] of Object.entries(errorMessages)) {
			const key = fn.name;
			expect(key, `code ${code} maps to an unnamed function`).not.toBe('');
			expect(en, `en.json is missing key "${key}" (code ${code})`).toHaveProperty(key);
			expect(nlNl, `nl-nl.json is missing key "${key}" (code ${code})`).toHaveProperty(key);
		}
	});

	it('uses the structured "<Category><Group>-<Sequence>" code format', () => {
		const codePattern = /^[A-Z]{1,3}\d{2}-\d{4}$/;
		for (const code of Object.keys(errorMessages)) {
			expect(code).toMatch(codePattern);
		}
	});
});

describe('resolveErrorMessage', () => {
	it('resolves a known code to its translated message', () => {
		expect(resolveErrorMessage('CF01-0001')).toBe(en.error_username_taken);
	});

	it('falls back to the generic message for an unrecognized code', () => {
		expect(resolveErrorMessage('Z99-9999')).toBe(en.error_generic);
	});

	it('falls back to the generic message when no code is given', () => {
		expect(resolveErrorMessage(undefined)).toBe(en.error_generic);
	});
});
