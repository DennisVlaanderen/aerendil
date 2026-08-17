import { m } from '$lib/paraglide/messages.js';

// Mirrors backend/internal/api/error_codes.go verbatim -- one constant per
// backend `Code*` value, same "<Category><Group>-<Sequence>" strings, same
// grouping. This is the single place a literal code string should appear
// anywhere in ui/src; bff routes import these for their own
// locally-synthesized errors (missing permission, missing token, request
// validation) instead of re-typing the string, and errorMessages below
// keys off the same constants.
export const ErrorCode = {
	// A01 -- authentication (401).
	AuthInvalidCredentials: 'A01-0001',
	AuthInvalidToken: 'A01-0002',
	AuthMissingHeader: 'A01-0003',

	// A02 -- authorization / access denied (403).
	AuthForbidden: 'A02-0001',

	// A03 -- protected/business-rule restriction (403).
	BusinessLastAdmin: 'A03-0001',
	BusinessProtectedGroup: 'A03-0002',
	BusinessLastEnvironment: 'A03-0003',
	BusinessEnvironmentHasFlags: 'A03-0004',
	BusinessAdminGroupChange: 'A03-0005',
	BusinessAdminOnlyUserDelete: 'A03-0006',
	BusinessEnvironmentHasCredentials: 'A03-0007',

	// BR01 -- bad request, general (400).
	BadRequestBody: 'BR01-0001',

	// BR02 -- bad request, flags domain (400).
	BadRequestFlagsEnvironmentIDRequired: 'BR02-0001',
	BadRequestFlagsKeyRequired: 'BR02-0002',
	BadRequestFlagsEnvironmentIDsRequired: 'BR02-0003',

	// BR03 -- bad request, environments domain (400).
	BadRequestEnvironmentUnknown: 'BR03-0001',
	BadRequestEnvironmentNameRequired: 'BR03-0002',

	// BR04 -- bad request, users domain (400).
	BadRequestUsernameRequired: 'BR04-0001',
	BadRequestPasswordRequired: 'BR04-0002',
	BadRequestPasswordTooShort: 'BR04-0003',
	BadRequestPasswordTooLong: 'BR04-0004',
	BadRequestUnknownGroupID: 'BR04-0005',

	// BR05 -- bad request, groups domain (400).
	BadRequestGroupNameRequired: 'BR05-0001',
	BadRequestUnknownPermission: 'BR05-0002',

	// BR06 -- bad request, application credentials domain (400).
	BadRequestCredentialNameRequired: 'BR06-0001',
	BadRequestCredentialEnvironmentRequired: 'BR06-0002',
	BadRequestUnknownScope: 'BR06-0003',

	// NF -- not found (404), one group per domain.
	NotFoundUser: 'NF01-0001',
	NotFoundFlag: 'NF02-0001',
	NotFoundEnvironment: 'NF03-0001',
	NotFoundGroup: 'NF04-0001',
	NotFoundApplicationCredential: 'NF05-0001',

	// CF -- conflict (409), one group per domain.
	ConflictUsernameTaken: 'CF01-0001',

	// MN01 -- method not allowed (405).
	MethodNotAllowed: 'MN01-0001',

	// IE01 -- internal error (500). Never synthesized locally by bff
	// routes, but listed for parity with the backend source of truth.
	InternalGeneric: 'IE01-0000',
	InternalTokenGen: 'IE01-0001',
	InternalPasswordHash: 'IE01-0002',
	InternalAuditFailed: 'IE01-0003',
	InternalClientSecretHash: 'IE01-0004'
} as const;

export type ErrorCode = (typeof ErrorCode)[keyof typeof ErrorCode];

// Maps a backend/bff error `code` to a translated message. bff routes reuse
// the same backend codes for their own locally-synthesized errors wherever
// the meaning matches, so this single map covers both sources.
//
// Internal-error codes (IE01-*) are deliberately absent -- they fall
// through to the generic fallback below rather than getting their own
// translated text, since there's nothing actionable to tell the user
// beyond "something went wrong".
// Exported (not just used internally) so errors.test.ts can assert every
// mapped code has a real translation key in both locales without
// duplicating this list.
export const errorMessages: Record<string, () => string> = {
	[ErrorCode.AuthInvalidCredentials]: m.error_invalid_credentials,
	[ErrorCode.AuthInvalidToken]: m.error_invalid_session,
	[ErrorCode.AuthMissingHeader]: m.error_missing_auth_header,
	[ErrorCode.AuthForbidden]: m.error_forbidden,
	[ErrorCode.BusinessLastAdmin]: m.error_last_admin,
	[ErrorCode.BusinessProtectedGroup]: m.error_protected_group,
	[ErrorCode.BusinessLastEnvironment]: m.error_last_environment,
	[ErrorCode.BusinessEnvironmentHasFlags]: m.error_environment_has_flags,
	[ErrorCode.BusinessAdminGroupChange]: m.error_admin_group_change_forbidden,
	[ErrorCode.BusinessAdminOnlyUserDelete]: m.error_admin_only_user_delete,
	[ErrorCode.BusinessEnvironmentHasCredentials]: m.error_environment_has_credentials,
	[ErrorCode.BadRequestBody]: m.error_invalid_body,
	[ErrorCode.BadRequestFlagsEnvironmentIDRequired]: m.error_environment_id_required,
	[ErrorCode.BadRequestFlagsKeyRequired]: m.error_flag_key_required,
	[ErrorCode.BadRequestFlagsEnvironmentIDsRequired]: m.error_flag_environments_required,
	[ErrorCode.BadRequestEnvironmentUnknown]: m.error_unknown_environment,
	[ErrorCode.BadRequestEnvironmentNameRequired]: m.error_environment_name_required,
	[ErrorCode.BadRequestUsernameRequired]: m.error_username_required,
	[ErrorCode.BadRequestPasswordRequired]: m.error_password_required,
	[ErrorCode.BadRequestPasswordTooShort]: m.error_password_too_short,
	[ErrorCode.BadRequestPasswordTooLong]: m.error_password_too_long,
	[ErrorCode.BadRequestUnknownGroupID]: m.error_unknown_group_id,
	[ErrorCode.BadRequestGroupNameRequired]: m.error_group_name_required,
	[ErrorCode.BadRequestUnknownPermission]: m.error_unknown_permission,
	[ErrorCode.BadRequestCredentialNameRequired]: m.error_credential_name_required,
	[ErrorCode.BadRequestCredentialEnvironmentRequired]: m.error_credential_environment_required,
	[ErrorCode.BadRequestUnknownScope]: m.error_unknown_scope,
	[ErrorCode.NotFoundUser]: m.error_user_not_found,
	[ErrorCode.NotFoundFlag]: m.error_flag_not_found,
	[ErrorCode.NotFoundEnvironment]: m.error_environment_not_found,
	[ErrorCode.NotFoundGroup]: m.error_group_not_found,
	[ErrorCode.NotFoundApplicationCredential]: m.error_application_credential_not_found,
	[ErrorCode.ConflictUsernameTaken]: m.error_username_taken,
	[ErrorCode.MethodNotAllowed]: m.error_method_not_allowed
};

// resolveErrorMessage translates an API/bff error code into the user's
// locale, falling back to a generic translated message for a missing or
// unrecognized code -- this is the only path a component should use to
// turn a failed apiRequest()/ApiResult into displayed text, so raw
// (untranslated) backend/bff message strings never reach the DOM.
export function resolveErrorMessage(code?: string): string {
	return (code ? errorMessages[code]?.() : undefined) ?? m.error_generic();
}
