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
	// Scopes comes back nil after Apply (omitempty drops an empty slice in
	// the Raft-log JSON encoding) -- normalize to non-nil, like toUserResponse.
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
// client secret, returned only on create and rotate -- list/get/update
// responses always use the plain applicationCredentialResponse.
type applicationCredentialSecretResponse struct {
	applicationCredentialResponse
	ClientSecret string `json:"clientSecret"`
}

// generateClientSecret returns a random OAuth2 client_secret: 32 bytes of
// crypto/rand, base64url-encoded (unpadded) so it embeds safely in a
// Basic-auth header or form value.
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

// validateCredentialScopes rejects any scope not in auth.CredentialScopes --
// credentials are deliberately restricted to a subset of the full
// permission catalog a human Group can hold.
func validateCredentialScopes(scopes []string) error {
	for _, scope := range scopes {
		if !auth.IsKnownCredentialScope(scope) {
			return badRequest(CodeBadRequestUnknownScope, "unknown scope: "+scope)
		}
	}
	return nil
}

// applicationCredentialsGetHandler requires an environmentId query param
// and returns only credentials scoped to it, same contract as
// flagsGetHandler -- a credential belongs to exactly one environment
// (AC-7.3), so hasEnvironmentAccess can gate it the same way.
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

	// Checked as soon as the target environment is known, before any other
	// field validation -- an unauthorized caller learns nothing else about
	// the payload.
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

	// Pointers so an omitted field means "unchanged" rather than decoding as
	// false/nil and wiping it -- an omitted "active" would otherwise revoke
	// a live credential's access on its next token exchange.
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

	// PUT never touches the secret (rotate is a separate operation) or the
	// environment, which is fixed for the credential's lifetime (AC-7.3) --
	// a live token re-resolves EnvironmentID on every request, so
	// reassigning it here would silently change access with no re-issuance.
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

// applicationCredentialsRotateHandler issues a new client secret,
// overwriting the old ClientSecretHash immediately without changing the
// credential's ID/client_id, name, environment, or scopes.
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
