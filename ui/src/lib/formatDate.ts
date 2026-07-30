import { getLocale } from '$lib/paraglide/runtime';

// AuditEntry.timestamp is unix seconds (see backend/internal/store/audit.go).
export function formatTimestamp(unixSeconds: number): string {
	return new Intl.DateTimeFormat(getLocale(), {
		dateStyle: 'medium',
		timeStyle: 'short'
	}).format(new Date(unixSeconds * 1000));
}
