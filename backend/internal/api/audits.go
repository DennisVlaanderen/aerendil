package api

import (
	"net/http"
	"strconv"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerAuditRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audits", requirePermission(auth.PermAuditsRead, handleErrors(auditsGetHandler)))
	mux.HandleFunc("GET /api/audits/{id}", requirePermission(auth.PermAuditsRead, handleErrors(auditsGetByIDHandler)))
}

func auditsGetHandler(w http.ResponseWriter, r *http.Request) error {
	filter := store.AuditFilter{
		TargetType: r.URL.Query().Get("targetType"),
		TargetID:   r.URL.Query().Get("targetId"),
		ActorID:    r.URL.Query().Get("actorId"),
	}
	return ok(w, map[string]any{"audits": dataStore.Audits().List(filter)})
}

// auditsGetByIDHandler looks up a single entry by its uint64 ID (the Raft
// log index). A malformed id is a 400, not a 404 -- it can never name an
// entry, as opposed to a well-formed id that just isn't there.
func auditsGetByIDHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return badRequest(CodeBadRequestAuditIDInvalid, "audit id must be a non-negative integer")
	}
	entry, found := dataStore.Audits().Get(id)
	if !found {
		return notFound(CodeNotFoundAudit, MsgNotFoundAudit)
	}
	return ok(w, entry)
}
