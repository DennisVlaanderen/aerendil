package api

import (
	"net/http"
	"strings"

	"aerendil/backend/internal/store"
)

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

func environmentsGetHandler(w http.ResponseWriter, r *http.Request) error {
	environments := dataStore.Environments().List()
	resp := make([]environmentResponse, 0, len(environments))
	for _, e := range environments {
		resp = append(resp, toEnvironmentResponse(e))
	}
	return ok(w, map[string]any{"environments": resp})
}

func environmentsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest("invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest("name is required")
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
		return notFound("environment not found")
	}

	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest("invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest("name is required")
	}

	// Order is intentionally not accepted from the request body -- it's
	// fixed at creation time (see environmentsPostHandler); reordering is a
	// follow-up once environment-scoped flags exist to make it matter.
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
		return notFound("environment not found")
	}

	if err := dataStore.Environments().Delete(id); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}
