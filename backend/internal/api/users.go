package api

import (
	"net/http"
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users", requirePermission(auth.PermUsersRead, handleErrors(usersGetHandler)))
	mux.HandleFunc("POST /api/users", requirePermission(auth.PermUsersCreate, withAudit(auditConfig{
		Action:     "user.create",
		TargetType: "user",
	}, handleErrors(usersPostHandler))))
	mux.HandleFunc("GET /api/users/{id}", requirePermission(auth.PermUsersRead, handleErrors(usersGetByIDHandler)))
	mux.HandleFunc("PUT /api/users/{id}", requirePermission(auth.PermUsersUpdate, withAudit(auditConfig{
		Action:     "user.update",
		TargetType: "user",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			u, ok := dataStore.Users().Get(r.PathValue("id"))
			return toUserResponse(u), ok
		},
	}, handleErrors(usersPutHandler))))
	mux.HandleFunc("DELETE /api/users/{id}", requirePermission(auth.PermUsersDelete, withAudit(auditConfig{
		Action:     "user.delete",
		TargetType: "user",
		Before: func(r *http.Request, _ []byte) (any, bool) {
			u, ok := dataStore.Users().Get(r.PathValue("id"))
			return toUserResponse(u), ok
		},
	}, handleErrors(usersDeleteHandler))))
}

// minPasswordLength is deliberately simple (length only, no complexity
// rules) -- this is an internal admin tool, not a public consumer app.
const minPasswordLength = 8

// maxPasswordLength mirrors bcrypt's own 72-byte input limit -- rejecting an
// oversized password here gives a clear 400 instead of letting
// bcrypt.GenerateFromPassword fail and fall through to a generic 500.
const maxPasswordLength = 72

// userResponse never includes PasswordHash -- password hashes never leave
// the store/auth layers.
type userResponse struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	GroupIDs []string `json:"groupIds"`
	Active   bool     `json:"active"`
}

func toUserResponse(u store.User) userResponse {
	// GroupIDs comes back nil after Apply (omitempty drops an empty slice in
	// the Raft-log JSON encoding) -- normalize so clients always see a real
	// array, never null.
	groupIDs := u.GroupIDs
	if groupIDs == nil {
		groupIDs = []string{}
	}
	return userResponse{
		ID:       u.ID,
		Username: u.Username,
		GroupIDs: groupIDs,
		Active:   u.Active,
	}
}

func usersGetHandler(w http.ResponseWriter, r *http.Request) error {
	users := dataStore.Users().List()
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	return ok(w, map[string]any{"users": resp})
}

func usersGetByIDHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	user, found := dataStore.Users().Get(id)
	if !found {
		return notFound(CodeNotFoundUser, MsgNotFoundUser)
	}
	return ok(w, toUserResponse(user))
}

func usersPostHandler(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Username string   `json:"username"`
		Password string   `json:"password"`
		GroupIDs []string `json:"groupIds"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	username := strings.ToLower(strings.TrimSpace(payload.Username))
	if username == "" {
		return badRequest(CodeBadRequestUsernameRequired, MsgBadRequestUsernameRequired)
	}
	if payload.Password == "" {
		return badRequest(CodeBadRequestPasswordRequired, "password is required")
	}
	if len(payload.Password) < minPasswordLength {
		return badRequest(CodeBadRequestPasswordTooShort, MsgBadRequestPasswordTooShort)
	}
	if len(payload.Password) > maxPasswordLength {
		return badRequest(CodeBadRequestPasswordTooLong, MsgBadRequestPasswordTooLong)
	}

	if err := requireAdminForAdminGroupChange(r, payload.GroupIDs); err != nil {
		return err
	}

	// Fast pre-check; fsm.applyUser is the authoritative enforcement point
	// for username uniqueness (see store.ErrUsernameTaken).
	if _, exists := dataStore.Users().GetByUsername(username); exists {
		return conflict(CodeConflictUsernameTaken, MsgConflictUsernameTaken)
	}
	for _, groupID := range payload.GroupIDs {
		if _, ok := dataStore.Groups().Get(groupID); !ok {
			return badRequest(CodeBadRequestUnknownGroupID, unknownGroupIDMessage(groupID))
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		return internalError(CodeInternalPasswordHash, MsgInternalPasswordHash)
	}

	user, err := dataStore.Users().Set(store.User{
		ID:           store.NewID(),
		Username:     username,
		PasswordHash: hash,
		GroupIDs:     payload.GroupIDs,
		Active:       true,
	})
	if err != nil {
		return err
	}
	return created(w, toUserResponse(user))
}

// requireAdminForAdminGroupChange returns a 403 if any of groupSets
// includes the Admin group and the caller isn't already an admin --
// otherwise a caller with only users:create/update could grant or retain
// Admin membership without ever holding groups permissions or admin rights.
// Pass a user's current and/or proposed group lists as needed.
func requireAdminForAdminGroupChange(r *http.Request, groupSets ...[]string) error {
	touchesAdmin := false
	for _, groupIDs := range groupSets {
		if slices.Contains(groupIDs, store.AdminGroupID) {
			touchesAdmin = true
			break
		}
	}
	if !touchesAdmin {
		return nil
	}
	principal, _ := principalFromContext(r)
	if !principal.IsAdmin {
		return forbidden(CodeBusinessAdminGroupChange, "only an Admin can modify Admin group membership")
	}
	return nil
}

func usersPutHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, found := dataStore.Users().Get(id)
	if !found {
		return notFound(CodeNotFoundUser, MsgNotFoundUser)
	}

	var payload struct {
		Username string    `json:"username"`
		Password string    `json:"password"`
		GroupIDs *[]string `json:"groupIds"`
		Active   bool      `json:"active"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}

	username := strings.ToLower(strings.TrimSpace(payload.Username))
	if username == "" {
		return badRequest(CodeBadRequestUsernameRequired, MsgBadRequestUsernameRequired)
	}
	if payload.Password != "" && len(payload.Password) < minPasswordLength {
		return badRequest(CodeBadRequestPasswordTooShort, MsgBadRequestPasswordTooShort)
	}
	if payload.Password != "" && len(payload.Password) > maxPasswordLength {
		return badRequest(CodeBadRequestPasswordTooLong, MsgBadRequestPasswordTooLong)
	}

	// Nil groupIds means "unchanged" (*[]string distinguishes omitted from
	// explicitly cleared) -- a caller without groups:read can't render group
	// checkboxes but must still be able to edit a user's other fields.
	groupIDs := existing.GroupIDs
	if payload.GroupIDs != nil {
		groupIDs = *payload.GroupIDs
	}

	if err := requireAdminForAdminGroupChange(r, existing.GroupIDs, groupIDs); err != nil {
		return err
	}

	// Fast pre-check; fsm.applyUser is the authoritative enforcement point
	// for username uniqueness (see store.ErrUsernameTaken). Excludes this
	// user's own record so keeping the same username isn't a false conflict.
	if other, exists := dataStore.Users().GetByUsername(username); exists && other.ID != id {
		return conflict(CodeConflictUsernameTaken, MsgConflictUsernameTaken)
	}
	for _, groupID := range groupIDs {
		if _, ok := dataStore.Groups().Get(groupID); !ok {
			return badRequest(CodeBadRequestUnknownGroupID, unknownGroupIDMessage(groupID))
		}
	}

	// Empty password means "unchanged" -- the edit form never round-trips
	// the hash, and clearing a password isn't a supported operation.
	passwordHash := existing.PasswordHash
	if payload.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
		if err != nil {
			return internalError(CodeInternalPasswordHash, MsgInternalPasswordHash)
		}
		passwordHash = hash
	}

	user, err := dataStore.Users().Set(store.User{
		ID:           existing.ID,
		Username:     username,
		PasswordHash: passwordHash,
		GroupIDs:     groupIDs,
		Active:       payload.Active,
	})
	if err != nil {
		return err
	}
	return ok(w, toUserResponse(user))
}

func usersDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if _, found := dataStore.Users().Get(id); !found {
		return notFound(CodeNotFoundUser, MsgNotFoundUser)
	}

	// Hardcoded to admins only for now, on top of the users:delete check
	// above -- removing this block is all that's needed to relax it later.
	principal, _ := principalFromContext(r)
	if !principal.IsAdmin {
		return forbidden(CodeBusinessAdminOnlyUserDelete, "only an Admin can delete users")
	}

	if err := dataStore.Users().Delete(id); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}
