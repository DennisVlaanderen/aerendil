package api

import (
	"fmt"
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerGroupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/groups", requirePermission(auth.PermGroupsRead, handleErrors(groupsGetHandler)))
	mux.HandleFunc("POST /api/groups", requirePermission(auth.PermGroupsCreate, withAudit(auditConfig{
		Action:     "group.create",
		TargetType: "group",
	}, handleErrors(groupsPostHandler))))
	mux.HandleFunc("PUT /api/groups/{id}", requirePermission(auth.PermGroupsUpdate, withAudit(auditConfig{
		Action:     "group.update",
		TargetType: "group",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			g, ok := dataStore.Groups().Get(r.PathValue("id"))
			return toGroupResponse(g), ok
		},
	}, handleErrors(groupsPutHandler))))
	mux.HandleFunc("DELETE /api/groups/{id}", requirePermission(auth.PermGroupsDelete, withAudit(auditConfig{
		Action:     "group.delete",
		TargetType: "group",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			g, ok := dataStore.Groups().Get(r.PathValue("id"))
			return toGroupResponse(g), ok
		},
	}, handleErrors(groupsDeleteHandler))))
}

type groupResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Permissions    []string `json:"permissions"`
	EnvironmentIDs []string `json:"environmentIds"`
	System         bool     `json:"system"`
}

func toGroupResponse(g store.Group) groupResponse {
	// Permissions/EnvironmentIDs come back nil after Apply (omitempty drops
	// an empty slice in the Raft-log JSON encoding) -- normalize to non-nil
	// so clients always see a real array, never null.
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
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestGroupNameRequired, "name is required")
	}
	if err := validatePermissions(payload.Permissions); err != nil {
		return badRequest(CodeBadRequestUnknownPermission, err.Error())
	}
	if err := validateEnvironmentIDs(payload.EnvironmentIDs); err != nil {
		return badRequest(CodeBadRequestEnvironmentUnknown, err.Error())
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

	// No pre-check for the Admin group -- Set below is the source of truth
	// and returns ErrProtectedSystemGroup, which maps to the same 403.
	existing, found := dataStore.Groups().Get(id)
	if !found {
		return notFound(CodeNotFoundGroup, "group not found")
	}

	var payload struct {
		Name           string   `json:"name"`
		Permissions    []string `json:"permissions"`
		EnvironmentIDs []string `json:"environmentIds"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, "invalid request body")
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return badRequest(CodeBadRequestGroupNameRequired, "name is required")
	}
	if err := validatePermissions(payload.Permissions); err != nil {
		return badRequest(CodeBadRequestUnknownPermission, err.Error())
	}
	if err := validateEnvironmentIDs(payload.EnvironmentIDs); err != nil {
		return badRequest(CodeBadRequestEnvironmentUnknown, err.Error())
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

	// No pre-check for the Admin group -- see groupsPutHandler; Delete is
	// the source of truth.
	if _, found := dataStore.Groups().Get(id); !found {
		return notFound(CodeNotFoundGroup, "group not found")
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
