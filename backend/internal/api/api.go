// Package api implements Aerendil's HTTP API: flags, auth, users, groups,
// audits, environments, and application credentials. See RegisterRoutes.
package api

import (
	"encoding/json"
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

	registerAuthRoutes(mux)
	registerFlagRoutes(mux)
	registerUserRoutes(mux)
	registerGroupRoutes(mux)
	registerAuditRoutes(mux)
	registerEnvironmentRoutes(mux)
	registerOAuthRoutes(mux)
	registerApplicationCredentialRoutes(mux)
}

func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return ok(w, apiResponse{Status: "ok"})
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
