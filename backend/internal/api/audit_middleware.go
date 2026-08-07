package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"maps"
	"net/http"

	"aerendil/backend/internal/store"
)

// auditConfig describes one mutating route's audit metadata. Before is nil
// for pure-create routes (users/groups POST -- the ID is fresh every time,
// generated server-side inside the handler, so there is never a prior
// state); it is set for every route that can genuinely overwrite an
// existing record: users/groups PUT/DELETE (identified by path {id}) and
// flags POST, which is an upsert-by-key per environment (identified by the
// request body's "key" and "environmentIds" fields, not a path param).
type auditConfig struct {
	Action     string
	TargetType string
	Before     func(r *http.Request, body []byte) (any, bool)
}

// withAudit wraps next so every invocation -- success or rejection -- is
// durably recorded via dataStore.Audits().Append before the real response
// reaches the client. It must be nested INSIDE requirePermission in route
// registration (requirePermission(perm, withAudit(cfg, handler))) so
// principalFromContext is already populated when this runs.
//
// If Append itself fails, that is surfaced to the client as an error even
// if the underlying mutation already committed -- a deliberate tradeoff: an
// audit trail that can silently go missing is worse than an occasional
// false-negative response to an already-applied change.
func withAudit(cfg auditConfig, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
		r.Body = io.NopCloser(bytes.NewReader(body))

		var beforeJSON string
		if cfg.Before != nil {
			if v, ok := cfg.Before(r, body); ok {
				if b, err := json.Marshal(v); err == nil {
					beforeJSON = string(b)
				}
			}
		}

		rec := newAuditRecorder()
		next(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		success := status >= 200 && status < 300

		principal, _ := principalFromContext(r)
		actorID := ""
		if principal.User != nil {
			actorID = principal.User.ID
		}
		actorType := "user"
		if principal.IsService {
			actorType = "applicationCredential"
		}

		entry := store.AuditEntry{
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     cfg.Action,
			TargetType: cfg.TargetType,
			TargetID:   targetIDFrom(r, rec.body.Bytes()),
			Before:     beforeJSON,
			Success:    success,
			StatusCode: status,
		}
		if success {
			entry.After = redactSensitiveResponseFields(rec.body.Bytes())
		} else {
			entry.Error = extractErrorMessage(rec.body.Bytes())
		}

		if _, err := dataStore.Audits().Append(entry); err != nil {
			log.Printf("api: failed to record audit entry for %s %s (mutation status %d): %v", r.Method, r.URL.Path, status, err)
			writeError(w, http.StatusInternalServerError, CodeInternalAuditFailed, "the request may have been applied, but recording its audit trail failed; verify before retrying")
			return
		}

		maps.Copy(w.Header(), rec.header)
		w.WriteHeader(status)
		_, _ = w.Write(rec.body.Bytes())
	}
}

// auditRecorder buffers a handler's response instead of writing it straight
// through, so withAudit can record the audit entry (and decide whether the
// mutation counts as a success) before anything reaches the real client.
type auditRecorder struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func newAuditRecorder() *auditRecorder {
	return &auditRecorder{header: make(http.Header)}
}

func (a *auditRecorder) Header() http.Header { return a.header }

func (a *auditRecorder) WriteHeader(status int) {
	if a.wroteHeader {
		return
	}
	a.status = status
	a.wroteHeader = true
}

func (a *auditRecorder) Write(p []byte) (int, error) {
	if !a.wroteHeader {
		a.WriteHeader(http.StatusOK)
	}
	return a.body.Write(p)
}

// targetIDFrom resolves the audited entity's identity: the path {id} param
// for PUT/DELETE routes, else a best-effort "id" (users/groups POST) or
// "key" (flags POST) field sniffed from the buffered response body -- for
// flags POST specifically, the response is now {"flags": [...]} (one
// multi-environment create can span several Flag records sharing one key),
// so the key is read from the first element of that array instead of a
// top-level field. Empty for a rejected create, which never allocated an
// identity in the first place.
func targetIDFrom(r *http.Request, respBody []byte) string {
	if id := r.PathValue("id"); id != "" {
		return id
	}
	var probe struct {
		ID    string `json:"id"`
		Key   string `json:"key"`
		Flags []struct {
			Key string `json:"key"`
		} `json:"flags"`
	}
	_ = json.Unmarshal(respBody, &probe)
	if probe.ID != "" {
		return probe.ID
	}
	if probe.Key != "" {
		return probe.Key
	}
	if len(probe.Flags) > 0 {
		return probe.Flags[0].Key
	}
	return ""
}

// sensitiveResponseFields lists top-level JSON response fields that must
// never be persisted into the durable, widely-readable audit trail even
// though the handler legitimately returns them to the caller once -- today
// just applicationCredentialSecretResponse.ClientSecret (see
// application_credentials.go), which is otherwise "revealed exactly twice,
// never again". Add to this list rather than teaching withAudit about
// individual response shapes.
var sensitiveResponseFields = []string{"clientSecret"}

// redactSensitiveResponseFields returns body with any top-level field named
// in sensitiveResponseFields replaced by a redaction marker, for storage in
// AuditEntry.After. Falls back to the raw body if it isn't a JSON object, so
// a non-object success response (there are none today) still gets audited
// rather than silently dropped.
func redactSensitiveResponseFields(body []byte) string {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return string(body)
	}

	redacted := false
	for _, field := range sensitiveResponseFields {
		if _, ok := probe[field]; ok {
			probe[field] = json.RawMessage(`"[redacted]"`)
			redacted = true
		}
	}
	if !redacted {
		return string(body)
	}

	b, err := json.Marshal(probe)
	if err != nil {
		return string(body)
	}
	return string(b)
}

// extractErrorMessage pulls the "error" field every handler's error
// response already uses (see writeError/handleErrors), falling back to the
// raw body if it isn't in that shape.
func extractErrorMessage(body []byte) string {
	var probe struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &probe); err == nil && probe.Error != "" {
		return probe.Error
	}
	return string(body)
}
