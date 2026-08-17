// Package api implements Aerendil's HTTP API: flags, auth, users, groups,
// audits, environments, and application credentials. See RegisterRoutes.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

type apiResponse struct {
	Status string `json:"status"`
}

const devJWTSecret = "aerendil-dev-secret"

// authService and dataStore are package-level so every handler file can
// reach them without a receiver; authService is built in RegisterRoutes
// since it needs the store.
var authService *auth.Service
var dataStore *store.Store

func jwtSecretFromEnvironment() string {
	secret := strings.TrimSpace(os.Getenv("AERENDIL_JWT_SECRET"))
	if secret == "" {
		if isProductionEnvironment() {
			log.Fatal("AERENDIL_JWT_SECRET must be set when AERENDIL_ENV=production")
		}
		log.Println("AERENDIL_JWT_SECRET not set; using insecure development default")
		return devJWTSecret
	}
	return secret
}

// RegisterRoutes wires every API route onto mux, backed by s. Call it
// exactly once per process — it also initializes the package-level
// authService and dataStore every handler reads.
func RegisterRoutes(mux *http.ServeMux, s *store.Store) {
	dataStore = s
	authService = auth.NewService(jwtSecretFromEnvironment(), dataStore)

	mux.HandleFunc("/api/health", handleErrors(healthHandler))
	mux.HandleFunc("GET /api/flags", requirePermission(auth.PermFlagsRead, handleErrors(flagsGetHandler)))
	mux.HandleFunc("POST /api/flags", requirePermission(auth.PermFlagsWrite, withAudit(auditConfig{
		Action:     "flag.set",
		TargetType: "flag",
		Before: func(r *http.Request, body []byte) (any, bool) {
			var probe struct {
				Key            string   `json:"key"`
				EnvironmentIDs []string `json:"environmentIds"`
			}
			if err := json.Unmarshal(body, &probe); err != nil || probe.Key == "" || len(probe.EnvironmentIDs) == 0 {
				return nil, false
			}
			// One Get per environment: a multi-environment create needs
			// before-state for each one, not just the first.
			before := make([]store.Flag, 0, len(probe.EnvironmentIDs))
			for _, envID := range probe.EnvironmentIDs {
				if flag, ok := dataStore.Flags().Get(envID, probe.Key); ok {
					before = append(before, flag)
				}
			}
			if len(before) == 0 {
				return nil, false
			}
			return before, true
		},
	}, handleErrors(flagsPostHandler))))
	mux.HandleFunc("PUT /api/flags/{key}", requirePermission(auth.PermFlagsUpdate, withAudit(auditConfig{
		Action:     "flag.update",
		TargetType: "flag",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			f, ok := dataStore.Flags().Get(r.URL.Query().Get("environmentId"), r.PathValue("key"))
			return f, ok
		},
	}, handleErrors(flagsPutHandler))))
	mux.HandleFunc("DELETE /api/flags/{key}", requirePermission(auth.PermFlagsDelete, withAudit(auditConfig{
		Action:     "flag.delete",
		TargetType: "flag",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			f, ok := dataStore.Flags().Get(r.URL.Query().Get("environmentId"), r.PathValue("key"))
			return f, ok
		},
	}, handleErrors(flagsDeleteHandler))))
	mux.HandleFunc("/api/auth/login", handleErrors(loginHandler))
	mux.HandleFunc("/api/auth/me", handleErrors(meHandler))

	mux.HandleFunc("GET /api/users", requirePermission(auth.PermUsersRead, handleErrors(usersGetHandler)))
	mux.HandleFunc("POST /api/users", requirePermission(auth.PermUsersCreate, withAudit(auditConfig{
		Action:     "user.create",
		TargetType: "user",
	}, handleErrors(usersPostHandler))))
	mux.HandleFunc("PUT /api/users/{id}", requirePermission(auth.PermUsersUpdate, withAudit(auditConfig{
		Action:     "user.update",
		TargetType: "user",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			u, ok := dataStore.Users().Get(r.PathValue("id"))
			return toUserResponse(u), ok
		},
	}, handleErrors(usersPutHandler))))
	mux.HandleFunc("DELETE /api/users/{id}", requirePermission(auth.PermUsersDelete, withAudit(auditConfig{
		Action:     "user.delete",
		TargetType: "user",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			u, ok := dataStore.Users().Get(r.PathValue("id"))
			return toUserResponse(u), ok
		},
	}, handleErrors(usersDeleteHandler))))

	mux.HandleFunc("GET /api/groups", requirePermission(auth.PermGroupsRead, handleErrors(groupsGetHandler)))
	mux.HandleFunc("POST /api/groups", requirePermission(auth.PermGroupsCreate, withAudit(auditConfig{
		Action:     "group.create",
		TargetType: "group",
	}, handleErrors(groupsPostHandler))))
	mux.HandleFunc("PUT /api/groups/{id}", requirePermission(auth.PermGroupsUpdate, withAudit(auditConfig{
		Action:     "group.update",
		TargetType: "group",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			g, ok := dataStore.Groups().Get(r.PathValue("id"))
			return toGroupResponse(g), ok
		},
	}, handleErrors(groupsPutHandler))))
	mux.HandleFunc("DELETE /api/groups/{id}", requirePermission(auth.PermGroupsDelete, withAudit(auditConfig{
		Action:     "group.delete",
		TargetType: "group",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			g, ok := dataStore.Groups().Get(r.PathValue("id"))
			return toGroupResponse(g), ok
		},
	}, handleErrors(groupsDeleteHandler))))

	mux.HandleFunc("GET /api/audits", requirePermission(auth.PermAuditsRead, handleErrors(auditsGetHandler)))

	mux.HandleFunc("GET /api/environments", requirePermission(auth.PermEnvironmentsRead, handleErrors(environmentsGetHandler)))
	mux.HandleFunc("POST /api/environments", requirePermission(auth.PermEnvironmentsCreate, withAudit(auditConfig{
		Action:     "environment.create",
		TargetType: "environment",
	}, handleErrors(environmentsPostHandler))))
	mux.HandleFunc("PUT /api/environments/{id}", requirePermission(auth.PermEnvironmentsUpdate, withAudit(auditConfig{
		Action:     "environment.update",
		TargetType: "environment",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			e, ok := dataStore.Environments().Get(r.PathValue("id"))
			return toEnvironmentResponse(e), ok
		},
	}, handleErrors(environmentsPutHandler))))
	mux.HandleFunc("DELETE /api/environments/{id}", requirePermission(auth.PermEnvironmentsDelete, withAudit(auditConfig{
		Action:     "environment.delete",
		TargetType: "environment",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			e, ok := dataStore.Environments().Get(r.PathValue("id"))
			return toEnvironmentResponse(e), ok
		},
	}, handleErrors(environmentsDeleteHandler))))

	// Unprotected by design, like /api/auth/login. No "POST " prefix so a
	// wrong-method request gets oauthTokenHandler's RFC 6749 error body
	// instead of ServeMux's plain-text 405.
	mux.HandleFunc("/api/oauth/token", handleErrors(oauthTokenHandler))

	mux.HandleFunc("GET /api/application-credentials", requirePermission(auth.PermApplicationCredentialsRead, handleErrors(applicationCredentialsGetHandler)))
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

func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return ok(w, apiResponse{Status: "ok"})
}

func flagsGetHandler(w http.ResponseWriter, r *http.Request) error {
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, "environmentId is required")
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	return ok(w, map[string]any{"flags": dataStore.Flags().List(environmentID)})
}

func flagsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Key            string   `json:"key"`
		Enabled        bool     `json:"enabled"`
		Value          string   `json:"value"`
		EnvironmentIDs []string `json:"environmentIds"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}
	if strings.TrimSpace(payload.Key) == "" {
		return badRequest(CodeBadRequestFlagsKeyRequired, "key is required")
	}
	if len(payload.EnvironmentIDs) == 0 {
		return badRequest(CodeBadRequestFlagsEnvironmentIDsRequired, "environmentIds is required")
	}

	// All-or-nothing: one environment the caller can't touch rejects the
	// whole request, matching SetMany's atomic-across-environments intent.
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, "forbidden")
	}
	for _, envID := range payload.EnvironmentIDs {
		if !principal.hasEnvironmentAccess(envID) {
			return forbidden(CodeAuthForbidden, "forbidden")
		}
	}
	for _, envID := range payload.EnvironmentIDs {
		if _, exists := dataStore.Environments().Get(envID); !exists {
			return badRequest(CodeBadRequestEnvironmentUnknown, fmt.Sprintf("unknown environment: %q", envID))
		}
	}

	flags, err := dataStore.Flags().SetMany(payload.Key, payload.Enabled, payload.Value, payload.EnvironmentIDs)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"flags": flags})
}

func flagsPutHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, "environmentId is required")
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	if _, exists := dataStore.Flags().Get(environmentID, key); !exists {
		return notFound(CodeNotFoundFlag, "flag not found")
	}

	var payload struct {
		Enabled bool   `json:"enabled"`
		Value   string `json:"value"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	flag, err := dataStore.Flags().Set(store.Flag{
		EnvironmentID: environmentID,
		Key:           key,
		Enabled:       payload.Enabled,
		Value:         payload.Value,
	})
	if err != nil {
		return err
	}
	return ok(w, flag)
}

func flagsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, "environmentId is required")
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, "forbidden")
	}

	if _, exists := dataStore.Flags().Get(environmentID, key); !exists {
		return notFound(CodeNotFoundFlag, "flag not found")
	}

	if err := dataStore.Flags().Delete(environmentID, key); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return methodNotAllowed(CodeMethodNotAllowed, "method not allowed")
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	user, err := authService.Authenticate(payload.Username, payload.Password)
	if err != nil {
		return unauthorized(CodeAuthInvalidCredentials, "invalid username or password")
	}

	token, err := authService.GenerateToken(user)
	if err != nil {
		return internalError(CodeInternalTokenGen, "failed to create token")
	}

	return ok(w, map[string]any{
		"token": token,
		"user":  user,
	})
}

func meHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return methodNotAllowed(CodeMethodNotAllowed, "method not allowed")
	}

	principal, found := authenticateRequest(w, r)
	if !found {
		return nil
	}

	return ok(w, map[string]any{
		"user":         principal.User,
		"isAdmin":      principal.IsAdmin,
		"permissions":  principal.Perms.Keys(),
		"environments": resolveEnvironmentSummaries(principal),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// respond writes payload as the JSON body with status and returns nil, so a
// handler's success path can end with `return respond(w, status, payload)`.
// ok/created below cover the statuses handlers need today.
func respond(w http.ResponseWriter, status int, payload any) error {
	writeJSON(w, status, payload)
	return nil
}

// ok writes a 200 response with payload as the JSON body.
func ok(w http.ResponseWriter, payload any) error { return respond(w, http.StatusOK, payload) }

// created writes a 201 response with payload as the JSON body.
func created(w http.ResponseWriter, payload any) error {
	return respond(w, http.StatusCreated, payload)
}

// maxRequestBodyBytes caps decoded JSON request bodies; generous for this
// API's small payloads while preventing unbounded body reads.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// decodeJSON decodes r.Body into dst, capping the body size read via
// http.MaxBytesReader first so an oversized body fails the decode instead
// of being fully buffered.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}

// isProductionEnvironment reports whether AERENDIL_ENV=production, which
// turns insecure-default fallbacks (JWT secret, admin password) into hard
// startup failures instead of warnings.
func isProductionEnvironment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AERENDIL_ENV")), "production")
}
