package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerFlagRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/flags", requirePermission(auth.PermFlagsRead, handleErrors(flagsGetHandler)))
	mux.HandleFunc("POST /api/flags", requirePermission(auth.PermFlagsWrite, withAudit(auditConfig{
		Action:     "flag.set",
		TargetType: "flag",
		Before: func(_ *http.Request, body []byte) (any, bool) {
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
