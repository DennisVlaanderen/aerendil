import { defineEnvVars } from '@sveltejs/kit/env';

export const variables = defineEnvVars({
	AERENDIL_API_ORIGIN: {
		description: 'Origin of the Aerendil backend HTTP API, reached only from server-side code.',
		schema: (value) => value?.trim() || 'http://127.0.0.1:8080'
	}
});
