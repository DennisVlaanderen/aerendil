package api

// Messages for codes returned from more than one call site, so the text
// can't drift. Single-use messages stay inline until a second site appears.
const (
	// A01/A02 -- auth.
	MsgAuthForbidden = "forbidden"

	// BR01 -- bad request, general.
	MsgBadRequestBody          = "invalid request body"
	MsgBadRequestCursorInvalid = "cursor must be a value returned as nextCursor or prevCursor"

	// BR02 -- bad request, flags domain.
	MsgBadRequestFlagsEnvironmentIDRequired = "environmentId is required"

	// BR03 -- bad request, environments domain.
	MsgBadRequestEnvironmentNameRequired = "name is required"

	// BR04 -- bad request, users domain.
	MsgBadRequestUsernameRequired = "username is required"
	MsgBadRequestPasswordTooShort = "password must be at least 8 characters"
	MsgBadRequestPasswordTooLong  = "password must be at most 72 characters"

	// BR05 -- bad request, groups domain.
	MsgBadRequestGroupNameRequired = "name is required"

	// BR06 -- bad request, application credentials domain.
	MsgBadRequestCredentialEnvironmentRequired = "environmentId is required"

	// NF -- not found, one per domain.
	MsgNotFoundUser                  = "user not found"
	MsgNotFoundFlag                  = "flag not found"
	MsgNotFoundEnvironment           = "environment not found"
	MsgNotFoundGroup                 = "group not found"
	MsgNotFoundApplicationCredential = "application credential not found"
	MsgNotFoundAudit                 = "audit entry not found"

	// CF -- conflict.
	MsgConflictUsernameTaken = "username is already taken"

	// MN01 -- method not allowed.
	MsgMethodNotAllowed = "method not allowed"

	// IE01 -- internal error.
	//nolint:gosec // G101: error text, not a credential
	MsgInternalPasswordHash = "failed to hash password"
)

// unknownGroupIDMessage builds the CodeBadRequestUnknownGroupID message.
func unknownGroupIDMessage(id string) string {
	return "unknown group id: " + id
}
