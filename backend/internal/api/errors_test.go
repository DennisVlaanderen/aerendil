package api

import (
	"regexp"
	"testing"

	"aerendil/backend/internal/store"
)

// codePattern matches the "<Category><Group>-<Sequence>" structured code
// format documented in error_codes.go, e.g. "A01-0001" or "BR03-0002".
var codePattern = regexp.MustCompile(`^[A-Z]{1,3}\d{2}-\d{4}$`)

// TestStoreErrorToAPIErrorCodesAreStructured guards against a future store
// sentinel being added to storeErrorToAPIError without a matching
// structured code -- every case must carry one so the frontend can always
// translate it (see ui/src/lib/errors.ts).
func TestStoreErrorToAPIErrorCodesAreStructured(t *testing.T) {
	sentinels := []error{
		store.ErrUsernameTaken,
		store.ErrLastAdmin,
		store.ErrProtectedSystemGroup,
		store.ErrLastEnvironment,
		store.ErrEnvironmentHasFlags,
		store.ErrUnknownEnvironment,
	}

	for _, sentinel := range sentinels {
		mapped := storeErrorToAPIError(sentinel)
		if mapped == nil {
			t.Errorf("storeErrorToAPIError(%v) = nil, want a mapped apiError", sentinel)
			continue
		}
		if !codePattern.MatchString(mapped.code) {
			t.Errorf("storeErrorToAPIError(%v).code = %q, want it to match %s", sentinel, mapped.code, codePattern)
		}
	}
}
