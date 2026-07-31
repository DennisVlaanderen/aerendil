package api

import (
	"net/http"

	"aerendil/backend/internal/store"
)

func auditsGetHandler(w http.ResponseWriter, r *http.Request) error {
	filter := store.AuditFilter{
		TargetType: r.URL.Query().Get("targetType"),
		TargetID:   r.URL.Query().Get("targetId"),
		ActorID:    r.URL.Query().Get("actorId"),
	}
	return ok(w, map[string]any{"audits": dataStore.Audits().List(filter)})
}
