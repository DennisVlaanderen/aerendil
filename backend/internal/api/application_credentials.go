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

func applicationCredentialsGetHandler(w http.ResponseWriter, r *http.Request) error {
	creds := dataStore.ApplicationCredentials().List()
	resp := make([]applicationCredentialResponse, 0, len(creds))
	for _, c := range creds {
		resp = append(resp, toApplicationCredentialResponse(c))
	}
	return ok(w, map[string]any{"applicationCredentials": resp})
}

func applicationCredentialsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Name          string   `json:"name"`
		EnvironmentID string   `json:"environmentId"`
		Scopes        []string `json:"scopes"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestCredentialNameRequired, "name is required")
	}
	environmentID := strings.TrimSpace(payload.EnvironmentID)
	if environmentID == "" {
		return badRequest(CodeBadRequestCredentialEnvironmentRequired, "environmentId is required")
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
	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return notFound(CodeNotFoundApplicationCredential, "application credential not found")
	}

	var payload struct {
		Name          string   `json:"name"`
		EnvironmentID string   `json:"environmentId"`
		Scopes        []string `json:"scopes"`
		Active        bool     `json:"active"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestCredentialNameRequired, "name is required")
	}
	environmentID := strings.TrimSpace(payload.EnvironmentID)
	if environmentID == "" {
		return badRequest(CodeBadRequestCredentialEnvironmentRequired, "environmentId is required")
	}
	if _, exists := dataStore.Environments().Get(environmentID); !exists {
		return badRequest(CodeBadRequestEnvironmentUnknown, "unknown environment: "+environmentID)
	}
	if err := validateCredentialScopes(payload.Scopes); err != nil {
		return err
	}

	// PUT never touches the secret -- rotating it is a distinct operation
	// (applicationCredentialsRotateHandler) with its own "reveal the new
	// secret exactly once" response shape, the same way users.go's PUT
	// leaves PasswordHash alone unless a new password is explicitly sent.
	cred, err := dataStore.ApplicationCredentials().Set(store.ApplicationCredential{
		ID:               existing.ID,
		Name:             name,
		ClientSecretHash: existing.ClientSecretHash,
		EnvironmentID:    environmentID,
		Scopes:           payload.Scopes,
		Active:           payload.Active,
	})
	if err != nil {
		return err
	}
	return ok(w, toApplicationCredentialResponse(cred))
}

func applicationCredentialsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if _, found := dataStore.ApplicationCredentials().Get(id); !found {
		return notFound(CodeNotFoundApplicationCredential, "application credential not found")
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
	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return notFound(CodeNotFoundApplicationCredential, "application credential not found")
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
		return "", nil, internalError(CodeInternalPasswordHash, "failed to generate client secret")
	}
	hash, err = bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, internalError(CodeInternalPasswordHash, "failed to hash client secret")
	}
	return secret, hash, nil
}
