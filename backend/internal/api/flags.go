package api

import (
	"fmt"
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/eval"
	"aerendil/backend/internal/store"
)

func registerFlagRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/flags", requirePermission(auth.PermFlagsRead, handleErrors(flagsGetHandler)))
	mux.HandleFunc("GET /api/flags/{key}", requirePermission(auth.PermFlagsRead, handleErrors(flagsGetByKeyHandler)))
	// A dry run that writes nothing, so it needs read permission and isn't audited.
	mux.HandleFunc("POST /api/flags/{key}/evaluate", requirePermission(auth.PermFlagsRead, handleErrors(flagsEvaluateHandler)))
	// Create-only (ErrFlagExists -> 409), so there's never a Before state.
	mux.HandleFunc("POST /api/flags", requirePermission(auth.PermFlagsWrite, withAudit(auditConfig{
		Action:     "flag.set",
		TargetType: "flag",
		Details:    flagRuleChanges,
	}, handleErrors(flagsPostHandler))))
	mux.HandleFunc("PUT /api/flags/{key}", requirePermission(auth.PermFlagsUpdate, withAudit(auditConfig{
		Action:     "flag.update",
		TargetType: "flag",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			f, ok := dataStore.Flags().Get(r.URL.Query().Get("environmentId"), r.PathValue("key"))
			return f, ok
		},
		Details: flagRuleChanges,
	}, handleErrors(flagsPutHandler))))
	mux.HandleFunc("POST /api/flags/{key}/force-disable", requirePermission(auth.PermFlagsUpdate, withAudit(auditConfig{
		Action:     "flag.force_disable",
		TargetType: "flag",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			f, ok := dataStore.Flags().Get(strings.TrimSpace(r.URL.Query().Get("environmentId")), r.PathValue("key"))
			return f, ok
		},
	}, handleErrors(flagsForceDisableHandler))))
	mux.HandleFunc("DELETE /api/flags/{key}", requirePermission(auth.PermFlagsDelete, withAudit(auditConfig{
		Action:     "flag.delete",
		TargetType: "flag",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			f, ok := dataStore.Flags().Get(r.URL.Query().Get("environmentId"), r.PathValue("key"))
			return f, ok
		},
		Details: flagRuleChanges,
	}, handleErrors(flagsDeleteHandler))))
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

	flags, page, err := paginate(r, dataStore.Flags().List(environmentID))
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
	return ok(w, flag)
}

func flagsPostHandler(w http.ResponseWriter, r *http.Request) error {
	// The §6.1 definition plus the environments to create it in. Fields the
	// store owns (revision, schemaVersion, evaluatorLevel) are ignored.
	var payload struct {
		store.Flag
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
		// Per environment, since each has its own segments.
		payload.EnvironmentID = envID
		if err := validateFlag(&payload.Flag); err != nil {
			return err
		}
	}

	flags, err := dataStore.Flags().SetMany(payload.Flag, payload.EnvironmentIDs)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"flags": flags})
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

	if _, exists := dataStore.Flags().Get(environmentID, key); !exists {
		return notFound(CodeNotFoundFlag, MsgNotFoundFlag)
	}

	// The full §6.1 definition replaces the stored one. The path and query
	// name the flag; fields the store owns are ignored.
	var flag store.Flag
	if err := decodeJSON(w, r, &flag); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}
	flag.Key, flag.EnvironmentID = key, environmentID
	if err := validateFlag(&flag); err != nil {
		return err
	}

	applied, err := dataStore.Flags().Set(flag)
	if err != nil {
		return err
	}
	return ok(w, applied)
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

func flagsForceDisableHandler(w http.ResponseWriter, r *http.Request) error {
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

	existing.Enabled = false
	flag, err := dataStore.Flags().Set(existing)
	if err != nil {
		return err
	}
	return ok(w, flag)
}

// flagsEvaluateHandler runs eval.Evaluate on the stored flag for the context
// in the body, for dry runs from the UI and tests.
func flagsEvaluateHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if environmentID == "" {
		return badRequest(CodeBadRequestFlagsEnvironmentIDRequired, MsgBadRequestFlagsEnvironmentIDRequired)
	}

	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}

	var ctx eval.Context
	if err := decodeJSON(w, r, &ctx); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}
	if err := validateContext(ctx); err != nil {
		return err
	}

	stored, exists := dataStore.Flags().Get(environmentID, key)
	if !exists {
		return notFound(CodeNotFoundFlag, MsgNotFoundFlag)
	}
	flag, err := toEval[eval.Flag](stored)
	if err != nil {
		return err
	}
	segments, err := evalSegments(environmentID)
	if err != nil {
		return err
	}
	return ok(w, eval.Evaluate(&flag, segments, ctx))
}
