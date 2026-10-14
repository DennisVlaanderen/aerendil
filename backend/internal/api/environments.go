package api

import (
	"net/http"
	"regexp"
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
	Color string `json:"color"`
}

func toEnvironmentResponse(e store.Environment) environmentResponse {
	return environmentResponse{
		ID:    e.ID,
		Name:  e.Name,
		Order: e.Order,
		Color: e.Color,
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

// nextEnvironmentOrder returns one past the highest Order currently in
// use, so a new environment always sequences after every existing one.
// A plain count would reuse the Order of a still-present environment once
// any earlier one is deleted, letting two environments share an Order;
// listEnvironments then falls back to comparing random UUIDs, so their
// relative position is no longer creation order (see #52).
func nextEnvironmentOrder() int {
	next := 0
	for _, environment := range dataStore.Environments().List() {
		if environment.Order >= next {
			next = environment.Order + 1
		}
	}
	return next
}

func environmentsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestEnvironmentNameRequired, MsgBadRequestEnvironmentNameRequired)
	}

	color, colorValid := normalizeEnvironmentColor(payload.Color)
	if !colorValid {
		return badRequest(CodeBadRequestEnvironmentColorInvalid, MsgBadRequestEnvironmentColorInvalid)
	}

	env, err := dataStore.Environments().Set(store.Environment{
		ID:    store.NewID(),
		Name:  name,
		Order: nextEnvironmentOrder(),
		Color: color,
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
		Name  string  `json:"name"`
		Color *string `json:"color"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestEnvironmentNameRequired, MsgBadRequestEnvironmentNameRequired)
	}

	color := existing.Color
	if payload.Color != nil {
		normalized, colorValid := normalizeEnvironmentColor(*payload.Color)
		if !colorValid {
			return badRequest(CodeBadRequestEnvironmentColorInvalid, MsgBadRequestEnvironmentColorInvalid)
		}
		color = normalized
	}

	// Order is fixed at creation; reordering is a future follow-up.
	env, err := dataStore.Environments().Set(store.Environment{
		ID:    existing.ID,
		Name:  name,
		Order: existing.Order,
		Color: color,
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

var regexpEnvironmentColorPattern = regexp.MustCompile(`^([1-6]|#([0-9a-fA-F]{3}){1,2})$`)

func normalizeEnvironmentColor(raw string) (string, bool) {
	color := strings.TrimSpace(raw)
	if color == "" {
		return "", true
	}

	if regexpEnvironmentColorPattern.MatchString(color) {
		color = strings.ToLower(color)
		return color, true
	}

	return "", false
}
