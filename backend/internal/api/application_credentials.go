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

func registerApplicationCredentialRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/application-credentials", requirePermission(auth.PermApplicationCredentialsRead, handleErrors(applicationCredentialsGetHandler)))
	mux.HandleFunc("GET /api/application-credentials/{id}", requirePermission(auth.PermApplicationCredentialsRead, handleErrors(applicationCredentialsGetByIDHandler)))
	mux.HandleFunc("POST /api/application-credentials", requirePermission(auth.PermApplicationCredentialsCreate, withAudit(auditConfig{
		Action:     "applicationCredential.create",
		TargetType: "applicationCredential",
	}, handleErrors(applicationCredentialsPostHandler))))
	mux.HandleFunc("PUT /api/application-credentials/{id}", requirePermission(auth.PermApplicationCredentialsUpdate, withAudit(auditConfig{
		Action:     "applicationCredential.update",
		TargetType: "applicationCredential",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			c, ok := dataStore.ApplicationCredentials().Get(r.PathValue("id"))
			return toApplicationCredentialResponse(c), ok
		},
	}, handleErrors(applicationCredentialsPutHandler))))
	mux.HandleFunc("DELETE /api/application-credentials/{id}", requirePermission(auth.PermApplicationCredentialsDelete, withAudit(auditConfig{
		Action:     "applicationCredential.delete",
		TargetType: "applicationCredential",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			c, ok := dataStore.ApplicationCredentials().Get(r.PathValue("id"))
			return toApplicationCredentialResponse(c), ok
		},
	}, handleErrors(applicationCredentialsDeleteHandler))))
	mux.HandleFunc("POST /api/application-credentials/{id}/rotate", requirePermission(auth.PermApplicationCredentialsUpdate, withAudit(auditConfig{
		Action:     "applicationCredential.rotate",
		TargetType: "applicationCredential",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			c, ok := dataStore.ApplicationCredentials().Get(r.PathValue("id"))
			return toApplicationCredentialResponse(c), ok
		},
	}, handleErrors(applicationCredentialsRotateHandler))))
}

// applicationCredentialResponse omits ClientSecretHash; hashes never leave
// store/auth.
type applicationCredentialResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	EnvironmentID string   `json:"environmentId"`
	Scopes        []string `json:"scopes"`
	Active        bool     `json:"active"`
}

func toApplicationCredentialResponse(c store.ApplicationCredential) applicationCredentialResponse {
	// Apply returns nil for an empty slice (omitempty); serve [] not null.
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

// applicationCredentialSecretResponse adds the plaintext secret; only
// create and rotate return it.
type applicationCredentialSecretResponse struct {
	applicationCredentialResponse
	ClientSecret string `json:"clientSecret"`
}

// generateClientSecret returns 32 random bytes, unpadded base64url so it
// is safe in a Basic-auth header or form value.
func generateClientSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// applicationCredentialNotFound is the shared 404 for every {id} route.
func applicationCredentialNotFound() error {
	return notFound(CodeNotFoundApplicationCredential, MsgNotFoundApplicationCredential)
}

// validateCredentialName trims and requires Name; shared by POST and PUT.
func validateCredentialName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", badRequest(CodeBadRequestCredentialNameRequired, "name is required")
	}
	return name, nil
}

// validateCredentialScopes rejects scopes outside auth.CredentialScopes, a
// deliberate subset of group permissions.
func validateCredentialScopes(scopes []string) error {
	for _, scope := range scopes {
		if !auth.IsKnownCredentialScope(scope) {
			return badRequest(CodeBadRequestUnknownScope, "unknown scope: "+scope)
		}
	}
	return nil
}

// applicationCredentialsGetHandler lists one environment's credentials,
// like flagsGetHandler; a credential has exactly one environment (AC-7.3).
func applicationCredentialsGetHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestCredentialEnvironmentRequired, MsgBadRequestCredentialEnvironmentRequired)
	}
	if !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	creds := dataStore.ApplicationCredentials().List()
	resp := make([]applicationCredentialResponse, 0, len(creds))
	for _, c := range creds {
		if c.EnvironmentID != environmentID {
			continue
		}
		resp = append(resp, toApplicationCredentialResponse(c))
	}
	resp, page, err := paginate(r, resp)
	if err != nil {
		return err
	}
	return ok(w, listBody("applicationCredentials", resp, page))
}

func applicationCredentialsGetByIDHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	cred, found := dataStore.ApplicationCredentials().Get(r.PathValue("id"))
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(cred.EnvironmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}
	return ok(w, toApplicationCredentialResponse(cred))
}

func applicationCredentialsPostHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	var payload struct {
		Name          string   `json:"name"`
		EnvironmentID string   `json:"environmentId"`
		Scopes        []string `json:"scopes"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	// Checked before other validation so an unauthorized caller learns nothing.
	environmentID := strings.TrimSpace(payload.EnvironmentID)
	if environmentID == "" {
		return badRequest(CodeBadRequestCredentialEnvironmentRequired, MsgBadRequestCredentialEnvironmentRequired)
	}
	if !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
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
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(existing.EnvironmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	// Pointers so an omitted field means "unchanged"; an omitted "active"
	// would otherwise revoke the credential.
	var payload struct {
		Name   string    `json:"name"`
		Scopes *[]string `json:"scopes"`
		Active *bool     `json:"active"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
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

	// PUT never changes the secret (see rotate) or the environment (AC-7.3):
	// live tokens re-resolve it per request, so moving it would silently
	// change their access.
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
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(existing.EnvironmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	if err := dataStore.ApplicationCredentials().Delete(id); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}

// applicationCredentialsRotateHandler replaces the secret immediately;
// everything else is unchanged.
func applicationCredentialsRotateHandler(w http.ResponseWriter, r *http.Request) error {
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	id := r.PathValue("id")
	existing, found := dataStore.ApplicationCredentials().Get(id)
	if !found {
		return applicationCredentialNotFound()
	}
	if !principal.hasEnvironmentAccess(existing.EnvironmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
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

// newHashedClientSecret returns a plaintext secret (shown once) and its
// bcrypt hash (persisted).
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
