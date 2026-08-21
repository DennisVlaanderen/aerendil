import { resolve } from '$app/paths';
import { localizeHref } from '$lib/paraglide/runtime';
import type { ResolvedPathname } from '$app/types';

// `resolve()`'s generic overload distributes over every literal in the
// `Pathname` union; past ~20 routes TypeScript can no longer match a
// runtime-computed (non-literal) argument against it. Casting the function
// itself sidesteps that generic resolution instead of the argument.
const resolveUntyped = resolve as unknown as (path: string) => ResolvedPathname;

/** Localizes an app-relative path and resolves it against the base path. */
export function localizedResolve(path: string): ResolvedPathname {
	return resolveUntyped(localizeHref(path));
}
