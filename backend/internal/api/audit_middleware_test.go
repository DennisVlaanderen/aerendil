package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func TestAuditRecorder(t *testing.T) {
	t.Run("implicit 200 when handler only calls Write", func(t *testing.T) {
		rec := newAuditRecorder()
		n, err := rec.Write([]byte("hello"))
		if err != nil || n != 5 {
			t.Fatalf("expected Write to succeed with 5 bytes, got n=%d err=%v", n, err)
		}
		if rec.status != http.StatusOK || !rec.wroteHeader {
			t.Fatalf("expected implicit 200 status and wroteHeader=true, got status=%d wroteHeader=%v", rec.status, rec.wroteHeader)
		}
		if rec.body.String() != "hello" {
			t.Fatalf("expected buffered body %q, got %q", "hello", rec.body.String())
		}
	})

	t.Run("multiple Write calls append to body and preserve status", func(t *testing.T) {
		rec := newAuditRecorder()
		rec.WriteHeader(http.StatusCreated)
		_, _ = rec.Write([]byte("part-1:"))
		_, _ = rec.Write([]byte("part-2"))

		if rec.status != http.StatusCreated {
			t.Fatalf("expected status 201 to be preserved across Write calls, got %d", rec.status)
		}
		if rec.body.String() != "part-1:part-2" {
			t.Fatalf("expected concatenated body %q, got %q", "part-1:part-2", rec.body.String())
		}
	})

	t.Run("WriteHeader called more than once keeps the first status", func(t *testing.T) {
		rec := newAuditRecorder()
		rec.Header().Set("X-Test", "1")
		rec.WriteHeader(http.StatusAccepted)
		rec.WriteHeader(http.StatusBadRequest)

		if rec.status != http.StatusAccepted {
			t.Fatalf("expected first status 202 to win, got %d", rec.status)
		}
		if rec.Header().Get("X-Test") != "1" {
			t.Fatalf("expected header X-Test=1, got %q", rec.Header().Get("X-Test"))
		}
	})
}

func TestTargetIDFrom(t *testing.T) {
	t.Run("ID taken from path value id", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/users/user-42", nil)
		req.SetPathValue("id", "user-42")
		req.SetPathValue("key", "ignored-key")

		if got := targetIDFrom(req, []byte(`{"id":"body-id"}`)); got != "user-42" {
			t.Fatalf("expected path id %q, got %q", "user-42", got)
		}
	})

	t.Run("ID taken from path value key", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/flags/flag-alpha", nil)
		req.SetPathValue("key", "flag-alpha")

		if got := targetIDFrom(req, []byte(`{"key":"body-key"}`)); got != "flag-alpha" {
			t.Fatalf("expected path key %q, got %q", "flag-alpha", got)
		}
	})

	t.Run("ID taken from response body id, key, or flags slice", func(t *testing.T) {
		cases := []struct {
			name string
			body string
			want string
		}{
			{"top-level id", `{"id":"env-99","name":"Prod"}`, "env-99"},
			{"top-level key", `{"key":"feature-x"}`, "feature-x"},
			{"flags array first element key", `{"flags":[{"key":"multi-env-flag"},{"key":"second"}]}`, "multi-env-flag"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/resource", nil)
				if got := targetIDFrom(req, []byte(tc.body)); got != tc.want {
					t.Fatalf("expected %q, got %q", tc.want, got)
				}
			})
		}
	})

	t.Run("no ID available returns empty string", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/resource", nil)
		for _, body := range []string{"", "not-json", `{"error":"bad request"}`, `{"flags":[]}`} {
			if got := targetIDFrom(req, []byte(body)); got != "" {
				t.Fatalf("expected empty target ID for body %q, got %q", body, got)
			}
		}
	})
}

func TestRedactSensitiveResponseFields(t *testing.T) {
	t.Run("redacts every configured sensitive field while preserving non-sensitive fields", func(t *testing.T) {
		origFields := slices.Clone(sensitiveResponseFields)
		sensitiveResponseFields = append(slices.Clone(sensitiveResponseFields), "passwordHash")
		t.Cleanup(func() { sensitiveResponseFields = origFields })

		raw := `{"id":"cred-1","name":"deploy","clientSecret":"raw-client-val","passwordHash":"raw-hash-val"}`
		redacted := redactSensitiveResponseFields([]byte(raw))

		if strings.Contains(redacted, "raw-client-val") || strings.Contains(redacted, "raw-hash-val") {
			t.Fatalf("expected sensitive values to be absent from redacted output, got %s", redacted)
		}

		var decoded map[string]string
		if err := json.Unmarshal([]byte(redacted), &decoded); err != nil {
			t.Fatalf("decode redacted JSON: %v", err)
		}
		for _, field := range sensitiveResponseFields {
			if decoded[field] != "[redacted]" {
				t.Fatalf("expected field %q to be [redacted], got %q", field, decoded[field])
			}
		}
		if decoded["id"] != "cred-1" || decoded["name"] != "deploy" {
			t.Fatalf("expected non-sensitive fields preserved, got %+v", decoded)
		}
	})

	t.Run("returns original body when no sensitive fields are present or body is not a JSON object", func(t *testing.T) {
		for _, body := range []string{`{"id":"u-1","username":"alice"}`, "plain-text-body", `[1,2,3]`, ""} {
			if got := redactSensitiveResponseFields([]byte(body)); got != body {
				t.Fatalf("expected %q unchanged, got %q", body, got)
			}
		}
	})
}

func TestExtractErrorMessage(t *testing.T) {
	t.Run("JSON error body returns error field", func(t *testing.T) {
		body := []byte(`{"code":"E100","error":"flag key is required"}`)
		if got := extractErrorMessage(body); got != "flag key is required" {
			t.Fatalf("expected %q, got %q", "flag key is required", got)
		}
	})

	t.Run("JSON body without error field falls back to raw body", func(t *testing.T) {
		body := []byte(`{"message":"missing error field"}`)
		if got := extractErrorMessage(body); got != string(body) {
			t.Fatalf("expected fallback %q, got %q", string(body), got)
		}
	})

	t.Run("non-JSON body falls back to raw string", func(t *testing.T) {
		body := []byte("upstream gateway timeout")
		if got := extractErrorMessage(body); got != "upstream gateway timeout" {
			t.Fatalf("expected %q, got %q", "upstream gateway timeout", got)
		}
	})

	t.Run("empty body returns empty string", func(t *testing.T) {
		if got := extractErrorMessage(nil); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})
}

func TestWithAuditMiddlewareDirect(t *testing.T) {
	_ = newTestMux(t)

	t.Run("redacts sensitive fields from recorded audit entry while returning full response to client", func(t *testing.T) {
		var seenBody string
		h := withAudit(auditConfig{
			Action:     "applicationCredential.create",
			TargetType: "applicationCredential",
			Before: func(_ *http.Request, body []byte) (any, bool) {
				seenBody = string(body)
				return map[string]string{"prev": "state"}, true
			},
		}, func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if string(b) != `{"name":"ci-bot"}` {
				t.Errorf("expected re-wrapped request body in handler, got %q", string(b))
			}
			w.Header().Set("X-Custom", "audit-ok")
			_, _ = w.Write([]byte(`{"id":"cred-77","name":"ci-bot","clientSecret":"top-secret-val"}`))
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/application-credentials", strings.NewReader(`{"name":"ci-bot"}`))
		req = withPrincipal(req, resolvedPrincipal{
			User:      &auth.User{ID: "svc-principal-1"},
			IsService: true,
		})
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Custom") != "audit-ok" {
			t.Fatalf("expected header X-Custom copied to response writer, got %q", rec.Header().Get("X-Custom"))
		}
		if !strings.Contains(rec.Body.String(), "top-secret-val") {
			t.Fatalf("expected client response to retain one-time secret, got %s", rec.Body.String())
		}
		if seenBody != `{"name":"ci-bot"}` {
			t.Fatalf("expected Before hook to receive request body, got %q", seenBody)
		}

		entries := dataStore.Audits().List(store.AuditFilter{TargetID: "cred-77"})
		if len(entries) != 1 {
			t.Fatalf("expected 1 audit entry for cred-77, got %d", len(entries))
		}
		entry := entries[0]
		if !entry.Success || entry.StatusCode != http.StatusOK {
			t.Fatalf("expected successful 200 audit entry, got %+v", entry)
		}
		if entry.ActorID != "svc-principal-1" || entry.ActorType != "applicationCredential" {
			t.Fatalf("expected service principal attribution, got actorID=%q actorType=%q", entry.ActorID, entry.ActorType)
		}
		if entry.Before != `{"prev":"state"}` {
			t.Fatalf("expected Before JSON %q, got %q", `{"prev":"state"}`, entry.Before)
		}
		if strings.Contains(entry.After, "top-secret-val") || !strings.Contains(entry.After, `"[redacted]"`) {
			t.Fatalf("expected clientSecret redacted in audit entry After, got %s", entry.After)
		}
	})

	t.Run("implicit 200 when handler writes nothing and Before returns false", func(t *testing.T) {
		h := withAudit(auditConfig{
			Action:     "environment.delete",
			TargetType: "environment",
			Before: func(_ *http.Request, _ []byte) (any, bool) {
				return nil, false
			},
		}, func(_ http.ResponseWriter, _ *http.Request) {})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/environments/env-silent", nil)
		req.SetPathValue("id", "env-silent")
		req = withPrincipal(req, resolvedPrincipal{
			User: &auth.User{ID: "user-1"},
		})
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected implicit 200, got %d", rec.Code)
		}
		entries := dataStore.Audits().List(store.AuditFilter{TargetID: "env-silent"})
		if len(entries) != 1 || !entries[0].Success || entries[0].Before != "" || entries[0].ActorType != "user" {
			t.Fatalf("unexpected audit entry for implicit 200: %+v", entries)
		}
	})

	t.Run("failed 4xx and 5xx responses record Success=false and error message", func(t *testing.T) {
		cases := []struct {
			name     string
			targetID string
			status   int
			rawBody  string
			wantErr  string
		}{
			{
				name:     "4xx JSON error response",
				targetID: "flag-400",
				status:   http.StatusBadRequest,
				rawBody:  `{"code":"E400","error":"invalid flag variation"}`,
				wantErr:  "invalid flag variation",
			},
			{
				name:     "5xx plain text error response",
				targetID: "flag-500",
				status:   http.StatusInternalServerError,
				rawBody:  "database unavailable",
				wantErr:  "database unavailable",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				h := withAudit(auditConfig{
					Action:     "flag.set",
					TargetType: "flag",
				}, func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.rawBody))
				})

				req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/flags/"+tc.targetID, nil)
				req.SetPathValue("key", tc.targetID)
				rec := httptest.NewRecorder()
			h(rec, req)

				if rec.Code != tc.status {
					t.Fatalf("expected status %d, got %d", tc.status, rec.Code)
				}
				entries := dataStore.Audits().List(store.AuditFilter{TargetID: tc.targetID})
				if len(entries) != 1 {
					t.Fatalf("expected 1 audit entry for %s, got %d", tc.targetID, len(entries))
				}
				entry := entries[0]
				if entry.Success || entry.StatusCode != tc.status || entry.After != "" || entry.Error != tc.wantErr {
					t.Fatalf("unexpected failed audit entry: %+v", entry)
				}
			})
		}
	})

	t.Run("returns 500 CodeInternalAuditFailed when audit store Append fails", func(t *testing.T) {
		dataDir, err := os.MkdirTemp("", "aerendil-audit-mw-follower-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

		follower, err := store.Open(store.Config{
			NodeID:    "audit-mw-follower",
			BindAddr:  "127.0.0.1:0",
			DataDir:   dataDir,
			Bootstrap: false,
		})
		if err != nil {
			t.Fatalf("open follower store: %v", err)
		}
		t.Cleanup(func() { _ = follower.Close() })

		origStore := dataStore
		dataStore = follower
		t.Cleanup(func() { dataStore = origStore })

		h := withAudit(auditConfig{
			Action:     "flag.set",
			TargetType: "flag",
		}, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"key":"unrecorded"}`))
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/flags", bytes.NewReader([]byte(`{}`)))
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on audit Append failure, got %d: %s", rec.Code, rec.Body.String())
		}
		assertErrorBody(t, rec, CodeInternalAuditFailed)
	})
}
