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
	mux.HandleFunc("GET /api/flags/{key}", requirePermission(auth.PermFlagsRead, handleErrors(flagsGetByKeyHandler)))
	// Create-only (ErrFlagExists -> 409), so there's never a Before state.
	mux.HandleFunc("POST /api/flags", requirePermission(auth.PermFlagsWrite, withAudit(auditConfig{
		Action:     "flag.set",
		TargetType: "flag",
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

// shortcut: legacyFlag keeps the pre-§6.1 {enabled, value} HTTP contract on
// top of the definition model, as a one-variation string flag; #146 replaces
// it with the §6.1 JSON.
type legacyFlag struct {
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Enabled       bool   `json:"enabled"`
	Value         string `json:"value,omitempty"`
	Version       uint64 `json:"version"`
}

const legacyVariationID = "value"

func legacyDefinition(environmentID, key string, enabled bool, value string) store.Flag {
	raw, _ := json.Marshal(value) // marshaling a string can't fail
	return store.Flag{
		EnvironmentID: environmentID,
		Key:           key,
		Type:          store.FlagTypeString,
		Enabled:       enabled,
		Variations:    []store.Variation{{ID: legacyVariationID, Value: raw}},
		OffVariation:  legacyVariationID,
		Fallthrough:   store.Serve{Variation: legacyVariationID},
	}
}

func toLegacyFlag(flag store.Flag) legacyFlag {
	var value string
	if len(flag.Variations) > 0 {
		_ = json.Unmarshal(flag.Variations[0].Value, &value)
	}
	return legacyFlag{
		EnvironmentID: flag.EnvironmentID,
		Key:           flag.Key,
		Enabled:       flag.Enabled,
		Value:         value,
		Version:       flag.Revision,
	}
}

func toLegacyFlags(flags []store.Flag) []legacyFlag {
	legacy := make([]legacyFlag, len(flags))
	for i, flag := range flags {
		legacy[i] = toLegacyFlag(flag)
	}
	return legacy
}

func flagsGetHandler(w http.ResponseWriter, r *http.Request) error {
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, MsgBadRequestFlagsEnvironmentIDRequired)
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	flags, page, err := paginate(r, toLegacyFlags(dataStore.Flags().List(environmentID)))
	if err != nil {
		return err
	}
	return ok(w, listBody("flags", flags, page))
}

func flagsGetByKeyHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, MsgBadRequestFlagsEnvironmentIDRequired)
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	flag, exists := dataStore.Flags().Get(environmentID, key)
	if !exists {
		return notFound(CodeNotFoundFlag, MsgNotFoundFlag)
	}
	return ok(w, toLegacyFlag(flag))
}

func flagsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Key            string   `json:"key"`
		Enabled        bool     `json:"enabled"`
		Value          string   `json:"value"`
		EnvironmentIDs []string `json:"environmentIds"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}
	if strings.TrimSpace(payload.Key) == "" {
		return badRequest(CodeBadRequestFlagsKeyRequired, "key is required")
	}
	if len(payload.EnvironmentIDs) == 0 {
		return badRequest(CodeBadRequestFlagsEnvironmentIDsRequired, "environmentIds is required")
	}

	// All-or-nothing, matching SetMany's atomicity.
	principal, found := principalFromContext(r)
	if !found {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}
	for _, envID := range payload.EnvironmentIDs {
		if !principal.hasEnvironmentAccess(envID) {
			return forbidden(CodeAuthForbidden, MsgAuthForbidden)
		}

		if _, exists := dataStore.Environments().Get(envID); !exists {
			return badRequest(CodeBadRequestEnvironmentUnknown, fmt.Sprintf("unknown environment: %q", envID))
		}
	}

	flags, err := dataStore.Flags().SetMany(legacyDefinition("", payload.Key, payload.Enabled, payload.Value), payload.EnvironmentIDs)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"flags": toLegacyFlags(flags)})
}

func flagsPutHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, MsgBadRequestFlagsEnvironmentIDRequired)
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	existing, exists := dataStore.Flags().Get(environmentID, key)
	if !exists {
		return notFound(CodeNotFoundFlag, MsgNotFoundFlag)
	}

	// Pointers so an omitted field means "unchanged"; an omitted "enabled"
	// would otherwise turn the flag off, and an omitted "value" would blank
	// it, as a side effect of updating the other one.
	var payload struct {
		Enabled *bool   `json:"enabled"`
		Value   *string `json:"value"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	enabled := existing.Enabled
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	value := toLegacyFlag(existing).Value
	if payload.Value != nil {
		value = *payload.Value
	}

	flag, err := dataStore.Flags().Set(legacyDefinition(environmentID, key, enabled, value))
	if err != nil {
		return err
	}
	return ok(w, toLegacyFlag(flag))
}

func flagsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, MsgBadRequestFlagsEnvironmentIDRequired)
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	if _, exists := dataStore.Flags().Get(environmentID, key); !exists {
		return notFound(CodeNotFoundFlag, MsgNotFoundFlag)
	}

	if err := dataStore.Flags().Delete(environmentID, key); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}
