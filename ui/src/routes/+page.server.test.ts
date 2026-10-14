import { describe, expect, it } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+page.server';

describe('root route', () => {
	it('redirects / to /login with a 303 for anonymous visitors', async () => {
		const cookies = { get: () => undefined } as unknown as Cookies;

		let thrown: unknown;
		try {
			await load({ cookies } as Parameters<typeof load>[0]);
		} catch (error) {
			thrown = error;
		}

		expect(isRedirect(thrown)).toBe(true);
		expect(thrown).toMatchObject({ status: 303, location: '/login' });
	});
});
