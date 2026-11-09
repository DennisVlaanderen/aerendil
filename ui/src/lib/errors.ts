import { m } from '#lib/paraglide/messages.js';

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
	BusinessEnvironmentHasSegments: 'A03-0008',

	// BR01 -- bad request, general (400).
	BadRequestBody: 'BR01-0001',
	BadRequestCursorInvalid: 'BR01-0002',

	// BR02 -- bad request, flags domain (400).
	BadRequestFlagsEnvironmentIDRequired: 'BR02-0001',
	BadRequestFlagsKeyRequired: 'BR02-0002',
	BadRequestFlagsEnvironmentIDsRequired: 'BR02-0003',
	BadRequestFlagKeyInvalid: 'BR02-0004',
	BadRequestFlagTypeInvalid: 'BR02-0005',
	BadRequestFlagVariationsRequired: 'BR02-0006',
	BadRequestFlagVariationValueInvalid: 'BR02-0007',
	BadRequestFlagBooleanVariations: 'BR02-0008',
	BadRequestFlagVariationUnknown: 'BR02-0009',
	BadRequestFlagServeInvalid: 'BR02-0010',
	BadRequestFlagRolloutWeightsInvalid: 'BR02-0011',
	BadRequestFlagTypeChanged: 'BR02-0012',
	BadRequestFlagContextInvalid: 'BR02-0013',

	// BR03 -- bad request, environments domain (400).
	BadRequestEnvironmentUnknown: 'BR03-0001',
	BadRequestEnvironmentNameRequired: 'BR03-0002',
	BadRequestEnvironmentColorInvalid: 'BR03-0003',

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

	// BR07 -- bad request, audits domain (400).
	BadRequestAuditIDInvalid: 'BR07-0001',
	BadRequestAuditCursorInvalid: 'BR07-0002',
	BadRequestAuditTimeRangeInvalid: 'BR07-0003',

	// BR08 -- bad request, the rule language flags and segments share (400).
	BadRequestDefinitionIDInvalid: 'BR08-0001',
	BadRequestDefinitionIDDuplicate: 'BR08-0002',
	BadRequestDefinitionLimitExceeded: 'BR08-0003',
	BadRequestConditionGroupInvalid: 'BR08-0004',
	BadRequestConditionGroupNested: 'BR08-0005',
	BadRequestConditionInvalid: 'BR08-0006',
	BadRequestConditionValuesInvalid: 'BR08-0007',
	BadRequestSegmentUnknown: 'BR08-0008',
	BadRequestSegmentUsesInSegment: 'BR08-0009',

	// BR09 -- bad request, segments domain (400).
	BadRequestSegmentKeyInvalid: 'BR09-0001',
	BadRequestSegmentEnvironmentIDRequired: 'BR09-0002',

	// NF -- not found (404), one group per domain.
	NotFoundUser: 'NF01-0001',
	NotFoundFlag: 'NF02-0001',
	NotFoundEnvironment: 'NF03-0001',
	NotFoundGroup: 'NF04-0001',
	NotFoundApplicationCredential: 'NF05-0001',
	NotFoundAudit: 'NF06-0001',
	NotFoundSegment: 'NF07-0001',

	// CF -- conflict (409), one group per domain.
	ConflictUsernameTaken: 'CF01-0001',
	ConflictFlagExists: 'CF02-0001',
	ConflictSegmentExists: 'CF03-0001',
	ConflictSegmentInUse: 'CF03-0002',

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
	[ErrorCode.BusinessEnvironmentHasSegments]: m.error_environment_has_segments,
	[ErrorCode.BadRequestBody]: m.error_invalid_body,
	[ErrorCode.BadRequestCursorInvalid]: m.error_cursor_invalid,
	[ErrorCode.BadRequestFlagsEnvironmentIDRequired]: m.error_environment_id_required,
	[ErrorCode.BadRequestFlagsKeyRequired]: m.error_flag_key_required,
	[ErrorCode.BadRequestFlagsEnvironmentIDsRequired]: m.error_flag_environments_required,
	[ErrorCode.BadRequestEnvironmentUnknown]: m.error_unknown_environment,
	[ErrorCode.BadRequestEnvironmentNameRequired]: m.error_environment_name_required,
	[ErrorCode.BadRequestEnvironmentColorInvalid]: m.error_environment_color_invalid,
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
	[ErrorCode.BadRequestAuditIDInvalid]: m.error_audit_id_invalid,
	[ErrorCode.BadRequestAuditCursorInvalid]: m.error_audit_cursor_invalid,
	[ErrorCode.BadRequestAuditTimeRangeInvalid]: m.error_audit_time_range_invalid,
	[ErrorCode.NotFoundUser]: m.error_user_not_found,
	[ErrorCode.NotFoundFlag]: m.error_flag_not_found,
	[ErrorCode.NotFoundEnvironment]: m.error_environment_not_found,
	[ErrorCode.NotFoundGroup]: m.error_group_not_found,
	[ErrorCode.NotFoundApplicationCredential]: m.error_application_credential_not_found,
	[ErrorCode.NotFoundAudit]: m.error_audit_not_found,
	[ErrorCode.ConflictUsernameTaken]: m.error_username_taken,
	[ErrorCode.ConflictFlagExists]: m.error_flag_exists,
	[ErrorCode.BadRequestFlagKeyInvalid]: m.error_flag_key_invalid,
	[ErrorCode.BadRequestFlagTypeInvalid]: m.error_flag_type_invalid,
	[ErrorCode.BadRequestFlagVariationsRequired]: m.error_flag_variations_required,
	[ErrorCode.BadRequestFlagVariationValueInvalid]: m.error_flag_variation_value_invalid,
	[ErrorCode.BadRequestFlagBooleanVariations]: m.error_flag_boolean_variations,
	[ErrorCode.BadRequestFlagVariationUnknown]: m.error_flag_variation_unknown,
	[ErrorCode.BadRequestFlagServeInvalid]: m.error_flag_serve_invalid,
	[ErrorCode.BadRequestFlagRolloutWeightsInvalid]: m.error_flag_rollout_weights_invalid,
	[ErrorCode.BadRequestFlagTypeChanged]: m.error_flag_type_changed,
	[ErrorCode.BadRequestFlagContextInvalid]: m.error_flag_context_invalid,
	[ErrorCode.BadRequestDefinitionIDInvalid]: m.error_definition_id_invalid,
	[ErrorCode.BadRequestDefinitionIDDuplicate]: m.error_definition_id_duplicate,
	[ErrorCode.BadRequestDefinitionLimitExceeded]: m.error_definition_limit_exceeded,
	[ErrorCode.BadRequestConditionGroupInvalid]: m.error_condition_group_invalid,
	[ErrorCode.BadRequestConditionGroupNested]: m.error_condition_group_nested,
	[ErrorCode.BadRequestConditionInvalid]: m.error_condition_invalid,
	[ErrorCode.BadRequestConditionValuesInvalid]: m.error_condition_values_invalid,
	[ErrorCode.BadRequestSegmentUnknown]: m.error_segment_unknown,
	[ErrorCode.BadRequestSegmentUsesInSegment]: m.error_segment_uses_in_segment,
	[ErrorCode.BadRequestSegmentKeyInvalid]: m.error_segment_key_invalid,
	[ErrorCode.BadRequestSegmentEnvironmentIDRequired]: m.error_segment_environment_required,
	[ErrorCode.NotFoundSegment]: m.error_segment_not_found,
	[ErrorCode.ConflictSegmentExists]: m.error_segment_exists,
	[ErrorCode.ConflictSegmentInUse]: m.error_segment_in_use,
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
