package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerAuditRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audits", requirePermission(auth.PermAuditsRead, handleErrors(auditsGetHandler)))
	mux.HandleFunc("GET /api/audits/{id}", requirePermission(auth.PermAuditsRead, handleErrors(auditsGetByIDHandler)))
}

func auditsGetHandler(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	limit := parseLimit(r)
	var before uint64
	if cursor := query.Get("cursor"); cursor != "" {
		var err error
		before, err = strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return badRequest(CodeBadRequestAuditCursorInvalid, "cursor must be a value returned as nextCursor")
		}
	}

	from, fromErr := parseTimeParam(query.Get("from"))
	to, toErr := parseTimeParam(query.Get("to"))
	if fromErr != nil || toErr != nil || (from != 0 && to != 0 && from > to) {
		return badRequest(CodeBadRequestAuditTimeRangeInvalid, "from and to must be RFC 3339 timestamps with from not after to")
	}

	all := dataStore.Audits().List(store.AuditFilter{
		TargetType: query.Get("targetType"),
		TargetID:   query.Get("targetId"),
		ActorID:    query.Get("actorId"),
		From:       from,
		To:         to,
	})
	// all is ID-descending; start at the first ID below the cursor.
	startIdx := 0
	if before != 0 {
		startIdx = sort.Search(len(all), func(i int) bool { return all[i].ID < before })
	}
	endIdx := min(startIdx+limit, len(all))
	entries := all[startIdx:endIdx]

	page := listPage{Limit: limit, Total: len(all)}
	if len(entries) > 0 {
		page.Start, page.End = startIdx+1, endIdx
	}
	if startIdx > 0 {
		// +1 so the previous page includes its newest entry.
		page.PrevCursor = strconv.FormatUint(all[max(0, startIdx-limit)].ID+1, 10)
	}
	if endIdx < len(all) {
		page.NextCursor = strconv.FormatUint(all[endIdx-1].ID, 10)
	}
	views := make([]auditEntryView, len(entries))
	for i, e := range entries {
		views[i] = newAuditEntryView(e)
	}
	return ok(w, map[string]any{"audits": views, "page": page})
}

// auditsGetByIDHandler looks up an entry by its Raft log index. A malformed
// id is a 400, not a 404: it can never name an entry.
func auditsGetByIDHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return badRequest(CodeBadRequestAuditIDInvalid, "audit id must be a non-negative integer")
	}
	entry, found := dataStore.Audits().Get(id)
	if !found {
		return notFound(CodeNotFoundAudit, MsgNotFoundAudit)
	}
	return ok(w, newAuditEntryView(entry))
}

// parseTimeParam parses RFC 3339 to unix seconds; "" is 0 (unbounded).
func parseTimeParam(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

// auditEntryView serves the stored Before/After JSON text as nested JSON.
type auditEntryView struct {
	store.AuditEntry
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

func newAuditEntryView(e store.AuditEntry) auditEntryView {
	return auditEntryView{AuditEntry: e, Before: nestedJSON(e.Before), After: nestedJSON(e.After)}
}

// nestedJSON returns the snapshot as raw JSON, or a JSON string if invalid.
func nestedJSON(snapshot string) json.RawMessage {
	snapshot = strings.TrimSpace(snapshot)
	if snapshot == "" {
		return nil
	}
	if json.Valid([]byte(snapshot)) {
		return json.RawMessage(snapshot)
	}
	b, _ := json.Marshal(snapshot) // marshaling a string cannot fail
	return b
}
