package api

import (
	"encoding/json"
	"errors"
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

// authService and dataStore are package-level so every handler file in
// this package can reach them without threading a receiver through every
// function -- authService needs the store, so it can only be constructed
// once RegisterRoutes receives it, not at package-init time.
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

func RegisterRoutes(mux *http.ServeMux, s *store.Store) {
	dataStore = s
	authService = auth.NewService(jwtSecretFromEnvironment(), dataStore)

	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("GET /api/flags", requirePermission(auth.PermFlagsRead, flagsGetHandler))
	mux.HandleFunc("POST /api/flags", requirePermission(auth.PermFlagsWrite, withAudit(auditConfig{
		Action:     "flag.set",
		TargetType: "flag",
		Before: func(r *http.Request, body []byte) (any, bool) {
			var probe struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(body, &probe); err != nil || probe.Key == "" {
				return nil, false
			}
			return dataStore.Flags().Get(probe.Key)
		},
	}, flagsPostHandler)))
	mux.HandleFunc("/api/auth/login", loginHandler)
	mux.HandleFunc("/api/auth/me", meHandler)

	mux.HandleFunc("GET /api/users", requirePermission(auth.PermUsersRead, usersGetHandler))
	mux.HandleFunc("POST /api/users", requirePermission(auth.PermUsersCreate, withAudit(auditConfig{
		Action:     "user.create",
		TargetType: "user",
	}, usersPostHandler)))
	mux.HandleFunc("PUT /api/users/{id}", requirePermission(auth.PermUsersUpdate, withAudit(auditConfig{
		Action:     "user.update",
		TargetType: "user",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			u, ok := dataStore.Users().Get(r.PathValue("id"))
			return toUserResponse(u), ok
		},
	}, usersPutHandler)))
	mux.HandleFunc("DELETE /api/users/{id}", requirePermission(auth.PermUsersDelete, withAudit(auditConfig{
		Action:     "user.delete",
		TargetType: "user",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			u, ok := dataStore.Users().Get(r.PathValue("id"))
			return toUserResponse(u), ok
		},
	}, usersDeleteHandler)))

	mux.HandleFunc("GET /api/groups", requirePermission(auth.PermGroupsRead, groupsGetHandler))
	mux.HandleFunc("POST /api/groups", requirePermission(auth.PermGroupsCreate, withAudit(auditConfig{
		Action:     "group.create",
		TargetType: "group",
	}, groupsPostHandler)))
	mux.HandleFunc("PUT /api/groups/{id}", requirePermission(auth.PermGroupsUpdate, withAudit(auditConfig{
		Action:     "group.update",
		TargetType: "group",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			g, ok := dataStore.Groups().Get(r.PathValue("id"))
			return toGroupResponse(g), ok
		},
	}, groupsPutHandler)))
	mux.HandleFunc("DELETE /api/groups/{id}", requirePermission(auth.PermGroupsDelete, withAudit(auditConfig{
		Action:     "group.delete",
		TargetType: "group",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			g, ok := dataStore.Groups().Get(r.PathValue("id"))
			return toGroupResponse(g), ok
		},
	}, groupsDeleteHandler)))

	mux.HandleFunc("GET /api/audits", requirePermission(auth.PermAuditsRead, auditsGetHandler))

	mux.HandleFunc("GET /api/environments", requirePermission(auth.PermEnvironmentsRead, environmentsGetHandler))
	mux.HandleFunc("POST /api/environments", requirePermission(auth.PermEnvironmentsCreate, withAudit(auditConfig{
		Action:     "environment.create",
		TargetType: "environment",
	}, environmentsPostHandler)))
	mux.HandleFunc("PUT /api/environments/{id}", requirePermission(auth.PermEnvironmentsUpdate, withAudit(auditConfig{
		Action:     "environment.update",
		TargetType: "environment",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			e, ok := dataStore.Environments().Get(r.PathValue("id"))
			return toEnvironmentResponse(e), ok
		},
	}, environmentsPutHandler)))
	mux.HandleFunc("DELETE /api/environments/{id}", requirePermission(auth.PermEnvironmentsDelete, withAudit(auditConfig{
		Action:     "environment.delete",
		TargetType: "environment",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			e, ok := dataStore.Environments().Get(r.PathValue("id"))
			return toEnvironmentResponse(e), ok
		},
	}, environmentsDeleteHandler)))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok"})
}

func flagsGetHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"flags": dataStore.Flags().List()})
}

func flagsPostHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
		Value   string `json:"value"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(payload.Key) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key is required"})
		return
	}

	flag, err := dataStore.Flags().Set(store.Flag{Key: payload.Key, Enabled: payload.Enabled, Value: payload.Value})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, flag)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, err := authService.Authenticate(payload.Username, payload.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	token, err := authService.GenerateToken(user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	principal, ok := authenticateRequest(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":        principal.User,
		"isAdmin":     principal.IsAdmin,
		"permissions": principal.Perms.Keys(),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeStoreError maps a store-layer error to an appropriate client
// response: known, client-facing errors get a specific status and message;
// anything else is logged server-side and returned as a generic 500 so
// internal error text (paths, raft/bolt internals, etc.) never reaches the
// client.
func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrUsernameTaken) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username is already taken"})
		return
	}
	if errors.Is(err, store.ErrLastAdmin) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, store.ErrProtectedSystemGroup) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, store.ErrLastEnvironment) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("api: internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
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

// isProductionEnvironment reports whether AERENDIL_ENV is set to
// "production" -- the switch that turns insecure-default fallbacks (JWT
// secret, admin password) into hard startup failures instead of warnings.
// Left unset, behavior is unchanged from before this flag existed.
func isProductionEnvironment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AERENDIL_ENV")), "production")
}
