import { m } from '$lib/paraglide/messages.js';

// Maps a backend/bff error `code` (the structured "<Category><Group>-
// <Sequence>" strings documented in backend/internal/api/error_codes.go,
// e.g. "A01-0001") to a translated message. bff routes reuse the same
// backend codes for their own locally-synthesized errors (missing
// permission, missing token, request validation) wherever the meaning
// matches, so this single map covers both sources.
//
// Internal-error codes (IE01-*) are deliberately absent -- they fall
// through to the generic fallback below rather than getting their own
// translated text, since there's nothing actionable to tell the user
// beyond "something went wrong".
// Exported (not just used internally) so errors.test.ts can assert every
// mapped code has a real translation key in both locales without
// duplicating this list.
export const errorMessages: Record<string, () => string> = {
	'A01-0001': m.error_invalid_credentials,
	'A01-0002': m.error_invalid_session,
	'A01-0003': m.error_missing_auth_header,
	'A02-0001': m.error_forbidden,
	'A03-0001': m.error_last_admin,
	'A03-0002': m.error_protected_group,
	'A03-0003': m.error_last_environment,
	'A03-0004': m.error_environment_has_flags,
	'A03-0005': m.error_admin_group_change_forbidden,
	'A03-0006': m.error_admin_only_user_delete,
	'BR01-0001': m.error_invalid_body,
	'BR02-0001': m.error_environment_id_required,
	'BR02-0002': m.error_flag_key_required,
	'BR02-0003': m.error_flag_environments_required,
	'BR03-0001': m.error_unknown_environment,
	'BR03-0002': m.error_environment_name_required,
	'BR04-0001': m.error_username_required,
	'BR04-0002': m.error_password_required,
	'BR04-0003': m.error_password_too_short,
	'BR04-0004': m.error_password_too_long,
	'BR04-0005': m.error_unknown_group_id,
	'BR05-0001': m.error_group_name_required,
	'BR05-0002': m.error_unknown_permission,
	'NF01-0001': m.error_user_not_found,
	'NF03-0001': m.error_environment_not_found,
	'NF04-0001': m.error_group_not_found,
	'CF01-0001': m.error_username_taken,
	'MN01-0001': m.error_method_not_allowed
};

// resolveErrorMessage translates an API/bff error code into the user's
// locale, falling back to a generic translated message for a missing or
// unrecognized code -- this is the only path a component should use to
// turn a failed apiRequest()/ApiResult into displayed text, so raw
// (untranslated) backend/bff message strings never reach the DOM.
export function resolveErrorMessage(code?: string): string {
	return (code ? errorMessages[code]?.() : undefined) ?? m.error_generic();
}
