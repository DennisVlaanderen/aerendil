package api

// Error codes are stable identifiers on every apiError so the UI can
// translate failures. Format "<Category><Group>-<Sequence>": Category is the
// constructor (A auth, BR badRequest, NF notFound, CF conflict, MN
// methodNotAllowed, PT payloadTooLarge, IE internal), Group (2 digits) the
// domain, Sequence (4 digits) the error. A code names what the user is told,
// so call sites sharing a message share its code.
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
	CodeBusinessEnvironmentHasSegments    = "A03-0008" // environment still has segments

	// BR01 -- bad request, general (400).
	CodeBadRequestBody          = "BR01-0001" // undecodable JSON body
	CodeBadRequestCursorInvalid = "BR01-0002" // list ?cursor= isn't one we issued

	// BR02 -- bad request, flags domain (400).
	CodeBadRequestFlagsEnvironmentIDRequired  = "BR02-0001" // ?environmentId= missing
	CodeBadRequestFlagsKeyRequired            = "BR02-0002"
	CodeBadRequestFlagsEnvironmentIDsRequired = "BR02-0003"
	CodeBadRequestFlagKeyInvalid              = "BR02-0004" // key doesn't match the §2.2 flag-key pattern
	CodeBadRequestFlagTypeInvalid             = "BR02-0005" // type isn't boolean, integer, string or json
	CodeBadRequestFlagVariationsRequired      = "BR02-0006" // no variations
	CodeBadRequestFlagVariationValueInvalid   = "BR02-0007" // a variation value doesn't fit the type
	CodeBadRequestFlagBooleanVariations       = "BR02-0008" // boolean variations aren't exactly true, false
	CodeBadRequestFlagVariationUnknown        = "BR02-0009" // offVariation, serve or rollout names no variation, or a rollout lists one twice
	CodeBadRequestFlagServeInvalid            = "BR02-0010" // serve isn't exactly one of variation or rollout
	CodeBadRequestFlagRolloutWeightsInvalid   = "BR02-0011" // a weight is outside 0-100000, or they don't sum to 100000
	CodeBadRequestFlagTypeChanged             = "BR02-0012" // PUT changes an existing flag's type
	CodeBadRequestFlagContextInvalid          = "BR02-0013" // evaluate: the context breaks §6.6

	// BR03 -- bad request, environments domain (400).
	CodeBadRequestEnvironmentUnknown      = "BR03-0001" // unknown environment ID
	CodeBadRequestEnvironmentNameRequired = "BR03-0002"
	CodeBadRequestEnvironmentColorInvalid = "BR03-0003" // not a valid hex color or palette number

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

	// BR08 -- bad request, the rule language flags and segments share (400).
	CodeBadRequestDefinitionIDInvalid     = "BR08-0001" // variation or rule id doesn't match the §2.2 id pattern
	CodeBadRequestDefinitionIDDuplicate   = "BR08-0002" // variation or rule id used twice
	CodeBadRequestDefinitionLimitExceeded = "BR08-0003" // breaks a §2.2 count limit
	CodeBadRequestConditionGroupInvalid   = "BR08-0004" // unknown match, or an empty group
	CodeBadRequestConditionGroupNested    = "BR08-0005" // nested group (evaluator level 1)
	CodeBadRequestConditionInvalid        = "BR08-0006" // unknown op, wrong attribute, or no values
	CodeBadRequestConditionValuesInvalid  = "BR08-0007" // mixed or unfitting value kinds, or not a string, number or bool
	CodeBadRequestSegmentUnknown          = "BR08-0008" // inSegment names no segment in the environment
	CodeBadRequestSegmentUsesInSegment    = "BR08-0009" // a segment rule uses inSegment

	// BR09 -- bad request, segments domain (400).
	CodeBadRequestSegmentKeyInvalid            = "BR09-0001" // key doesn't match the §2.2 flag-key pattern
	CodeBadRequestSegmentEnvironmentIDRequired = "BR09-0002"

	// NF -- not found (404), one group per domain.
	CodeNotFoundUser                  = "NF01-0001"
	CodeNotFoundFlag                  = "NF02-0001"
	CodeNotFoundEnvironment           = "NF03-0001"
	CodeNotFoundGroup                 = "NF04-0001"
	CodeNotFoundApplicationCredential = "NF05-0001"
	CodeNotFoundAudit                 = "NF06-0001"
	CodeNotFoundSegment               = "NF07-0001"

	// CF -- conflict (409), one group per domain.
	CodeConflictUsernameTaken = "CF01-0001"
	CodeConflictFlagExists    = "CF02-0001" // POST /api/flags: key already exists in a target environment
	CodeConflictSegmentExists = "CF03-0001" // POST /api/segments: key already exists in the environment
	CodeConflictSegmentInUse  = "CF03-0002" // DELETE /api/segments/{key}: a flag still references it

	// MN01 -- method not allowed (405).
	CodeMethodNotAllowed = "MN01-0001"

	// IE01 -- internal error (500).
	CodeInternalGeneric          = "IE01-0000" // handleErrors' fallback for unmapped errors
	CodeInternalTokenGen         = "IE01-0001" // failed to create a JWT
	CodeInternalPasswordHash     = "IE01-0002" // failed to hash a password
	CodeInternalAuditFailed      = "IE01-0003" // mutation applied, audit entry failed
	CodeInternalClientSecretHash = "IE01-0004" // failed to generate/hash a client secret

	// PT01 -- payload too large (413).
	CodePayloadTooLarge = "PT01-0001" // request body exceeds maxRequestBodyBytes
)
