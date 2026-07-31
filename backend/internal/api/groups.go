package api

import (
	"fmt"
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

type groupResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Permissions    []string `json:"permissions"`
	EnvironmentIDs []string `json:"environmentIds"`
	System         bool     `json:"system"`
}

func toGroupResponse(g store.Group) groupResponse {
	// g.Permissions/g.EnvironmentIDs are nil for a group with none assigned
	// (the "omitempty" on both store.Group fields drops an empty slice
	// entirely when the command is JSON-encoded for the Raft log, so they
	// come back nil after Apply, even if the original request sent an empty
	// array). Normalize both to non-nil slices here so API clients always
	// see a real array, never null -- mirrors toUserResponse's identical
	// fix for GroupIDs.
	permissions := g.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	environmentIDs := g.EnvironmentIDs
	if environmentIDs == nil {
		environmentIDs = []string{}
	}
	return groupResponse{
		ID:             g.ID,
		Name:           g.Name,
		Permissions:    permissions,
		EnvironmentIDs: environmentIDs,
		System:         g.System,
	}
}

func groupsGetHandler(w http.ResponseWriter, r *http.Request) error {
	groups := dataStore.Groups().List()
	resp := make([]groupResponse, 0, len(groups))
	for _, g := range groups {
		resp = append(resp, toGroupResponse(g))
	}
	return ok(w, map[string]any{"groups": resp})
}

func groupsPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Name           string   `json:"name"`
		Permissions    []string `json:"permissions"`
		EnvironmentIDs []string `json:"environmentIds"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest("invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest("name is required")
	}
	if err := validatePermissions(payload.Permissions); err != nil {
		return badRequest(err.Error())
	}
	if err := validateEnvironmentIDs(payload.EnvironmentIDs); err != nil {
		return badRequest(err.Error())
	}

	group, err := dataStore.Groups().Set(store.Group{
		ID:             store.NewID(),
		Name:           name,
		Permissions:    payload.Permissions,
		EnvironmentIDs: payload.EnvironmentIDs,
		System:         false, // a client can never create a second system group
	})
	if err != nil {
		return err
	}
	return created(w, toGroupResponse(group))
}

func groupsPutHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	// No pre-check for the Admin group here -- dataStore.Groups().Set below
	// is the single source of truth (System-flag protected) and returns
	// store.ErrProtectedSystemGroup, which handleErrors maps to the same
	// 403 a duplicated ID check here would have produced.
	existing, found := dataStore.Groups().Get(id)
	if !found {
		return notFound("group not found")
	}

	var payload struct {
		Name           string   `json:"name"`
		Permissions    []string `json:"permissions"`
		EnvironmentIDs []string `json:"environmentIds"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest("invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest("name is required")
	}
	if err := validatePermissions(payload.Permissions); err != nil {
		return badRequest(err.Error())
	}
	if err := validateEnvironmentIDs(payload.EnvironmentIDs); err != nil {
		return badRequest(err.Error())
	}

	group, err := dataStore.Groups().Set(store.Group{
		ID:             existing.ID,
		Name:           name,
		Permissions:    payload.Permissions,
		EnvironmentIDs: payload.EnvironmentIDs,
		System:         false,
	})
	if err != nil {
		return err
	}
	return ok(w, toGroupResponse(group))
}

func groupsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	// No pre-check for the Admin group here -- see the identical comment in
	// groupsPutHandler; dataStore.Groups().Delete is the single source of
	// truth.
	if _, found := dataStore.Groups().Get(id); !found {
		return notFound("group not found")
	}

	if err := dataStore.Groups().Delete(id); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}

func validatePermissions(perms []string) error {
	for _, p := range perms {
		if !auth.IsKnownPermission(p) {
			return fmt.Errorf("unknown permission: %q", p)
		}
	}
	return nil
}

// validateEnvironmentIDs mirrors validatePermissions -- each ID must
// resolve to an existing Environment, so a client can't grant a group
// access to a typo'd or already-deleted environment.
func validateEnvironmentIDs(ids []string) error {
	for _, id := range ids {
		if _, ok := dataStore.Environments().Get(id); !ok {
			return fmt.Errorf("unknown environment: %q", id)
		}
	}
	return nil
}
