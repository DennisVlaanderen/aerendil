package api

// Error codes are stable identifiers on every apiError so the UI can
// translate failures. Format "<Category><Group>-<Sequence>": Category is the
// constructor (A auth, BR badRequest, NF notFound, CF conflict, MN
// methodNotAllowed, IE internal), Group (2 digits) the domain, Sequence (4
// digits) the error. A code names what the user is told, so call sites
// sharing a message share its code.
const (
	// A01 -- authentication (401).
	CodeAuthInvalidCredentials = "A01-0001" // bad username/password
	CodeAuthInvalidToken       = "A01-0002" // bad bearer token, or its user is gone/inactive
	CodeAuthMissingHeader      = "A01-0003" // no Authorization header at all

	// A02 -- authorization / access denied (403).
	CodeAuthForbidden = "A02-0001" // missing permission or environment access

	// A03 -- protected/business-rule restriction (403).
	CodeBusinessLastAdmin                 = "A03-0001" // last remaining admin
	CodeBusinessProtectedGroup            = "A03-0002" // Admin group is protected
	CodeBusinessLastEnvironment           = "A03-0003" // last remaining environment
	CodeBusinessEnvironmentHasFlags       = "A03-0004" // environment still has flags
	CodeBusinessAdminGroupChange          = "A03-0005" // only Admins change Admin membership
	CodeBusinessAdminOnlyUserDelete       = "A03-0006" // only Admins delete users
	CodeBusinessEnvironmentHasCredentials = "A03-0007" // environment still has credentials

	// BR01 -- bad request, general (400).
	CodeBadRequestBody          = "BR01-0001" // undecodable JSON body
	CodeBadRequestCursorInvalid = "BR01-0002" // list ?cursor= isn't one we issued

	// BR02 -- bad request, flags domain (400).
	CodeBadRequestFlagsEnvironmentIDRequired  = "BR02-0001" // ?environmentId= missing
	CodeBadRequestFlagsKeyRequired            = "BR02-0002"
	CodeBadRequestFlagsEnvironmentIDsRequired = "BR02-0003"

	// BR03 -- bad request, environments domain (400).
	CodeBadRequestEnvironmentUnknown      = "BR03-0001" // unknown environment ID
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

	// BR07 -- bad request, audits domain (400).
	CodeBadRequestAuditIDInvalid        = "BR07-0001" // audit id isn't a uint64
	CodeBadRequestAuditCursorInvalid    = "BR07-0002" // ?cursor= isn't one we issued
	CodeBadRequestAuditTimeRangeInvalid = "BR07-0003" // ?from=/?to= not RFC 3339, or reversed

	// NF -- not found (404), one group per domain.
	CodeNotFoundUser                  = "NF01-0001"
	CodeNotFoundFlag                  = "NF02-0001"
	CodeNotFoundEnvironment           = "NF03-0001"
	CodeNotFoundGroup                 = "NF04-0001"
	CodeNotFoundApplicationCredential = "NF05-0001"
	CodeNotFoundAudit                 = "NF06-0001"

	// CF -- conflict (409), one group per domain.
	CodeConflictUsernameTaken = "CF01-0001"

	// MN01 -- method not allowed (405).
	CodeMethodNotAllowed = "MN01-0001"

	// IE01 -- internal error (500).
	CodeInternalGeneric          = "IE01-0000" // handleErrors' fallback for unmapped errors
	CodeInternalTokenGen         = "IE01-0001" // failed to create a JWT
	CodeInternalPasswordHash     = "IE01-0002" // failed to hash a password
	CodeInternalAuditFailed      = "IE01-0003" // mutation applied, audit entry failed
	CodeInternalClientSecretHash = "IE01-0004" // failed to generate/hash a client secret
)
