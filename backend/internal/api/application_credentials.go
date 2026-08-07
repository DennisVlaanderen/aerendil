package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

// applicationCredentialResponse never includes ClientSecretHash -- secret
// hashes never leave the store/auth layers, same discipline as
// userResponse never including PasswordHash.
type applicationCredentialResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	EnvironmentID string   `json:"environmentId"`
	Scopes        []string `json:"scopes"`
	Active        bool     `json:"active"`
}

func toApplicationCredentialResponse(c store.ApplicationCredential) applicationCredentialResponse {
	// c.Scopes is nil for a credential with no scopes (the "omitempty" on
	// store.ApplicationCredential.Scopes drops an empty slice entirely when
	// the command is JSON-encoded for the Raft log) -- normalize to a
	// non-nil slice here, mirroring toUserResponse's identical GroupIDs
	// normalization.
	scopes := c.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return applicationCredentialResponse{
		ID:            c.ID,
		Name:          c.Name,
		EnvironmentID: c.EnvironmentID,
		Scopes:        scopes,
		Active:        c.Active,
	}
}

// applicationCredentialSecretResponse additionally carries the plaintext
// client secret -- returned exactly twice in a credential's lifetime (create
// and rotate) and never again; list/get/update responses are always the
// plain applicationCredentialResponse.
type applicationCredentialSecretResponse struct {
	applicationCredentialResponse
	ClientSecret string `json:"clientSecret"`
}

// generateClientSecret returns a random, high-entropy secret suitable for
// an OAuth2 client_secret: 32 bytes of crypto/rand, base64url-encoded
// (unpadded) so it's safe to embed directly in a Basic-auth header or form
// value without further escaping.
func generateClientSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// applicationCredentialNotFound is the shared 404 for every route addressed
// by {id} (PUT/DELETE/rotate), so the message/code can't drift between them.
func applicationCredentialNotFound() error {
	return notFound(CodeNotFoundApplicationCredential, "application credential not found")
}

// validateCredentialName trims and validates a credential's Name field --
// shared by POST and PUT so the "name is required" rule can't drift between
// create and update.
func validateCredentialName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", badRequest(CodeBadRequestCredentialNameRequired, "name is required")
	}
	return name, nil
}

// validateCredentialScopes rejects any scope that isn't in
// auth.CredentialScopes -- application credentials are deliberately
// restricted to a subset of the full permission catalog a human Group can
// hold (see CredentialScopes' doc comment in auth/permissions.go).
func validateCredentialScopes(scopes []string) error {
	for _, scope := range scopes {
		if !auth.IsKnownCredentialScope(scope) {
			return badRequest(CodeBadRequestUnknownScope, "unknown scope: "+scope)
		}
	}
	return nil
}

// applicationCredentialsGetHandler requires an environmentId query param and
// only returns credentials scoped to it, the same contract flagsGetHandler
// already has -- a credential belongs to exactly one environment (AC-7.3),
// so there is never a reason to list across all of them at once, and this
// is what lets hasEnvironmentAccess gate the response the same way it gates
// flags.
func applicationCredentialsGetHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestCredentialEnvironmentRequired, "environmentId is required")
	}
	if !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	creds := dataStore.ApplicationCredentials().List()
	resp := make([]applicationCredentialResponse, 0, len(creds))
	for _, c := range creds {
		if c.EnvironmentID != environmentID {
			continue
		}
		resp = append(resp, toApplicationCredentialResponse(c))
	}
	return ok(w, map[string]any{"applicationCredentials": resp})
}

func applicationCredentialsPostHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	var payload struct {
		Name          string   `json:"name"`
		EnvironmentID string   `json:"environmentId"`
		Scopes        []string `json:"scopes"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	// Authorization is checked as soon as we know the target environment --
	// before any other field is validated -- so a caller without access to
	// that environment learns nothing about the rest of the payload (e.g.
	// "name is required", "unknown scope") ahead of being authorized for it.
	environmentID := strings.TrimSpace(payload.EnvironmentID)
	if environmentID == "" {
		return badRequest(CodeBadRequestCredentialEnvironmentRequired, "environmentId is required")
	}
	if !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	name, err := validateCredentialName(payload.Name)
	if err != nil {
		return err
	}
	if _, exists := dataStore.Environments().Get(environmentID); !exists {
		return badRequest(CodeBadRequestEnvironmentUnknown, "unknown environment: "+environmentID)
	}
	if err := validateCredentialScopes(payload.Scopes); err != nil {
		return err
	}

	secret, hash, err := newHashedClientSecret()
	if err != nil {
		return err
	}

	cred, err := dataStore.ApplicationCredentials().Set(store.ApplicationCredential{
		ID:               store.NewID(),
		Name:             name,
		ClientSecretHash: hash,
		EnvironmentID:    environmentID,
		Scopes:           payload.Scopes,
		Active:           true,
	})
	if err != nil {
		return err
	}
	return created(w, applicationCredentialSecretResponse{
		applicationCredentialResponse: toApplicationCredentialResponse(cred),
		ClientSecret:                  secret,
	})
}

func applicationCredentialsPutHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(existing.EnvironmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	// Scopes/Active are pointers so a client that omits either field leaves
	// it unchanged, instead of a bare bool/slice silently decoding a missing
	// field as false/nil and wiping it -- unlike users.go/groups.go's PUTs,
	// an omitted "active" here would silently revoke a live machine
	// credential's access on its next OAuth2 token exchange.
	var payload struct {
		Name   string    `json:"name"`
		Scopes *[]string `json:"scopes"`
		Active *bool     `json:"active"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	name, err := validateCredentialName(payload.Name)
	if err != nil {
		return err
	}

	scopes := existing.Scopes
	if payload.Scopes != nil {
		if err := validateCredentialScopes(*payload.Scopes); err != nil {
			return err
		}
		scopes = *payload.Scopes
	}

	active := existing.Active
	if payload.Active != nil {
		active = *payload.Active
	}

	// PUT never touches the secret or the environment. The secret is a
	// distinct operation (applicationCredentialsRotateHandler) with its own
	// "reveal the new secret exactly once" response shape, the same way
	// users.go's PUT leaves PasswordHash alone unless a new password is
	// explicitly sent. The environment is fixed for the credential's
	// lifetime (AC-7.3): a live service token re-resolves EnvironmentID on
	// every request (auth.Service.authenticateServicePrincipal), so
	// reassigning it here would silently change what an already-issued
	// token can access with no re-issuance.
	cred, err := dataStore.ApplicationCredentials().Set(store.ApplicationCredential{
		ID:               existing.ID,
		Name:             name,
		ClientSecretHash: existing.ClientSecretHash,
		EnvironmentID:    existing.EnvironmentID,
		Scopes:           scopes,
		Active:           active,
	})
	if err != nil {
		return err
	}
	return ok(w, toApplicationCredentialResponse(cred))
}

func applicationCredentialsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(existing.EnvironmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	if err := dataStore.ApplicationCredentials().Delete(id); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}

// applicationCredentialsRotateHandler issues a new client secret for an
// existing credential, invalidating the old one immediately (the old
// ClientSecretHash is simply overwritten) without changing the credential's
// ID/client_id, name, environment, or scopes.
func applicationCredentialsRotateHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(existing.EnvironmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	secret, hash, err := newHashedClientSecret()
	if err != nil {
		return err
	}

	cred, err := dataStore.ApplicationCredentials().Set(store.ApplicationCredential{
		ID:               existing.ID,
		Name:             existing.Name,
		ClientSecretHash: hash,
		EnvironmentID:    existing.EnvironmentID,
		Scopes:           existing.Scopes,
		Active:           existing.Active,
	})
	if err != nil {
		return err
	}
	return ok(w, applicationCredentialSecretResponse{
		applicationCredentialResponse: toApplicationCredentialResponse(cred),
		ClientSecret:                  secret,
	})
}

// newHashedClientSecret generates a plaintext client secret and its bcrypt
// hash together, since every caller (create, rotate) needs both: the
// plaintext to return exactly once, the hash to persist.
func newHashedClientSecret() (secret string, hash []byte, err error) {
	secret, err = generateClientSecret()
	if err != nil {
		return "", nil, internalError(CodeInternalClientSecretHash, "failed to generate client secret")
	}
	hash, err = bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, internalError(CodeInternalClientSecretHash, "failed to hash client secret")
	}
	return secret, hash, nil
}
