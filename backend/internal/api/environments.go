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

func environmentsGetHandler(w http.ResponseWriter, r *http.Request) {
	environments := dataStore.Environments().List()
	resp := make([]environmentResponse, 0, len(environments))
	for _, e := range environments {
		resp = append(resp, toEnvironmentResponse(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": resp})
}

func environmentsPostHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	env, err := dataStore.Environments().Set(store.Environment{
		ID:    store.NewID(),
		Name:  name,
		Order: len(dataStore.Environments().List()),
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEnvironmentResponse(env))
}

func environmentsPutHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, ok := dataStore.Environments().Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "environment not found"})
		return
	}

	var payload struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
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
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEnvironmentResponse(env))
}

func environmentsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, ok := dataStore.Environments().Get(id); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "environment not found"})
		return
	}

	if err := dataStore.Environments().Delete(id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
