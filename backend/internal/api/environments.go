package api

import (
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerEnvironmentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/environments", requirePermission(auth.PermEnvironmentsRead, handleErrors(environmentsGetHandler)))
	mux.HandleFunc("POST /api/environments", requirePermission(auth.PermEnvironmentsCreate, withAudit(auditConfig{
		Action:     "environment.create",
		TargetType: "environment",
	}, handleErrors(environmentsPostHandler))))
	mux.HandleFunc("GET /api/environments/{id}", requirePermission(auth.PermEnvironmentsRead, handleErrors(environmentsGetByIDHandler)))
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
}

type environmentResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

func toEnvironmentResponse(e store.Environment) environmentResponse {
	return environmentResponse{
		ID:    e.ID,
		Name:  e.Name,
		Order: e.Order,
	}
}

// resolveEnvironmentSummaries returns the environments principal sees in
// /api/auth/me (all for Admin), as records so names need no
// environments:read. Filters List() to keep its Order sorting.
func resolveEnvironmentSummaries(principal resolvedPrincipal) []environmentResponse {
	all := dataStore.Environments().List()
	resp := make([]environmentResponse, 0, len(all))
	for _, e := range all {
		if principal.IsAdmin || principal.Envs.Has(e.ID) {
			resp = append(resp, toEnvironmentResponse(e))
		}
	}
	return resp
}

func environmentsGetHandler(w http.ResponseWriter, r *http.Request) error {
	environments := dataStore.Environments().List()
	resp := make([]environmentResponse, 0, len(environments))
	for _, e := range environments {
		resp = append(resp, toEnvironmentResponse(e))
	}
	resp, page, err := paginate(r, resp)
	if err != nil {
		return err
	}
	return ok(w, listBody("environments", resp, page))
}

func environmentsGetByIDHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	env, found := dataStore.Environments().Get(id)
	if !found {
		return notFound(CodeNotFoundEnvironment, MsgNotFoundEnvironment)
	}
	return ok(w, toEnvironmentResponse(env))
}

func environmentsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestEnvironmentNameRequired, MsgBadRequestEnvironmentNameRequired)
	}

	env, err := dataStore.Environments().Set(store.Environment{
		ID:    store.NewID(),
		Name:  name,
		Order: len(dataStore.Environments().List()),
	})
	if err != nil {
		return err
	}
	return created(w, toEnvironmentResponse(env))
}

func environmentsPutHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	existing, found := dataStore.Environments().Get(id)
	if !found {
		return notFound(CodeNotFoundEnvironment, MsgNotFoundEnvironment)
	}

	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestEnvironmentNameRequired, MsgBadRequestEnvironmentNameRequired)
	}

	// Order is fixed at creation; reordering is a future follow-up.
	env, err := dataStore.Environments().Set(store.Environment{
		ID:    existing.ID,
		Name:  name,
		Order: existing.Order,
	})
	if err != nil {
		return err
	}
	return ok(w, toEnvironmentResponse(env))
}

func environmentsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	if _, found := dataStore.Environments().Get(id); !found {
		return notFound(CodeNotFoundEnvironment, MsgNotFoundEnvironment)
	}

	if err := dataStore.Environments().Delete(id); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}
