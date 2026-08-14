package api

// Error codes are stable, machine-readable identifiers on every apiError
// (see errors.go) so the UI can translate failures instead of showing raw
// English text. Format: "<Category><Group>-<Sequence>".
//
//   - Category matches the apiError constructor (A = auth, BR = badRequest,
//     NF = notFound, CF = conflict, MN = methodNotAllowed, IE = internal).
//   - Group (2 digits) sub-divides a category by domain.
//   - Sequence (4 digits) identifies the error within its group.
//
// A code is reused across call sites sharing the same message (e.g. every
// "invalid request body" site) -- it identifies what the user is told, not
// which line produced it.
const (
	// A01 -- authentication (401).
	CodeAuthInvalidCredentials = "A01-0001" // bad username/password
	CodeAuthInvalidToken       = "A01-0002" // missing/expired/malformed bearer token, or the user it names is gone/inactive
	CodeAuthMissingHeader      = "A01-0003" // no Authorization header at all

	// A02 -- authorization / access denied (403).
	CodeAuthForbidden = "A02-0001" // insufficient permission, or no access to the requested environment

	// A03 -- protected/business-rule restriction (403).
	CodeBusinessLastAdmin                 = "A03-0001" // cannot remove the last remaining admin account
	CodeBusinessProtectedGroup            = "A03-0002" // the Admin group is protected and cannot be modified/deleted
	CodeBusinessLastEnvironment           = "A03-0003" // cannot delete the last remaining environment
	CodeBusinessEnvironmentHasFlags       = "A03-0004" // cannot delete an environment that still has flags
	CodeBusinessAdminGroupChange          = "A03-0005" // only an Admin can modify Admin group membership
	CodeBusinessAdminOnlyUserDelete       = "A03-0006" // only an Admin can delete users
	CodeBusinessEnvironmentHasCredentials = "A03-0007" // cannot delete an environment that still has application credentials

	// BR01 -- bad request, general (400).
	CodeBadRequestBody = "BR01-0001" // malformed/undecodable JSON request body

	// BR02 -- bad request, flags domain (400).
	CodeBadRequestFlagsEnvironmentIDRequired  = "BR02-0001" // environmentId query param missing
	CodeBadRequestFlagsKeyRequired            = "BR02-0002"
	CodeBadRequestFlagsEnvironmentIDsRequired = "BR02-0003"

	// BR03 -- bad request, environments domain (400).
	CodeBadRequestEnvironmentUnknown      = "BR03-0001" // referenced environment ID doesn't exist
	CodeBadRequestEnvironmentNameRequired = "BR03-0002"

	// BR04 -- bad request, users domain (400).
	CodeBadRequestUsernameRequired = "BR04-0001"
	CodeBadRequestPasswordRequired = "BR04-0002"
	CodeBadRequestPasswordTooShort = "BR04-0003"
	CodeBadRequestPasswordTooLong  = "BR04-0004"
	CodeBadRequestUnknownGroupID   = "BR04-0005"

	// BR05 -- bad request, groups domain (400).
	CodeBadRequestGroupNameRequired = "BR05-0001"
	CodeBadRequestUnknownPermission = "BR05-0002"

	// BR06 -- bad request, application credentials domain (400).
	CodeBadRequestCredentialNameRequired        = "BR06-0001"
	CodeBadRequestCredentialEnvironmentRequired = "BR06-0002"
	CodeBadRequestUnknownScope                  = "BR06-0003"

	// NF -- not found (404), one group per domain.
	CodeNotFoundUser                  = "NF01-0001"
	CodeNotFoundEnvironment           = "NF03-0001"
	CodeNotFoundGroup                 = "NF04-0001"
	CodeNotFoundApplicationCredential = "NF05-0001"

	// CF -- conflict (409), one group per domain.
	CodeConflictUsernameTaken = "CF01-0001"

	// MN01 -- method not allowed (405).
	CodeMethodNotAllowed = "MN01-0001"

	// IE01 -- internal error (500).
	CodeInternalGeneric          = "IE01-0000" // handleErrors' catch-all fallback for an unmapped error
	CodeInternalTokenGen         = "IE01-0001" // failed to create a JWT
	CodeInternalPasswordHash     = "IE01-0002" // failed to hash a password
	CodeInternalAuditFailed      = "IE01-0003" // the mutation applied but recording its audit entry failed
	CodeInternalClientSecretHash = "IE01-0004" // failed to generate/hash an application credential's client secret
)
