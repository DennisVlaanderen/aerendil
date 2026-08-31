package api

// Client-facing message text for the codes in error_codes.go, defined once
// per code that is (or is expected to become) reused across call sites --
// same reasoning as the Code catalog itself: text lives here once so it
// can't drift between the two-plus places that return it. A code used at a
// single call site keeps its message inline until a second call site shows
// up; move it here at that point instead of copy-pasting the literal.
const (
	// A01/A02 -- auth.
	MsgAuthForbidden = "forbidden"

	// BR01 -- bad request, general.
	MsgBadRequestBody = "invalid request body"

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

	// CF -- conflict.
	MsgConflictUsernameTaken = "username is already taken"

	// MN01 -- method not allowed.
	MsgMethodNotAllowed = "method not allowed"

	// IE01 -- internal error.
	MsgInternalPasswordHash = "failed to hash password"
)

// unknownGroupIDMessage builds the CodeBadRequestUnknownGroupID message --
// kept as a function rather than a constant since the id is parameterized.
func unknownGroupIDMessage(id string) string {
	return "unknown group id: " + id
}
