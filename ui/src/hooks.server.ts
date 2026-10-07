import type { Handle } from '@sveltejs/kit/hooks';
import { getTextDirection } from '#lib/paraglide/runtime.js';
import { paraglideMiddleware } from '#lib/paraglide/server.js';

const handleParaglide: Handle = ({ event, resolve }) =>
	// event.request is read-only since SvelteKit 3; routing is already
	// de-localized by the reroute hook in hooks.ts, so the original request is fine.
	paraglideMiddleware(event.request, ({ locale }) =>
		resolve(event, {
			transformPageChunk: ({ html }) =>
				html
					.replace('%paraglide.lang%', locale)
					.replace('%paraglide.dir%', getTextDirection(locale))
		})
	);

export const handle: Handle = handleParaglide;
