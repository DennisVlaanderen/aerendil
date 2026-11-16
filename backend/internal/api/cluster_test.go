package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aerendil/backend/internal/auth"
)

func getClusterStatus(t *testing.T, mux *http.ServeMux, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/cluster/status", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestClusterStatusRequiresAuthAndPermission(t *testing.T) {
	mux := newTestMux(t)

	if rec := getClusterStatus(t, mux, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", rec.Code)
	}
	if rec := getClusterStatus(t, mux, tokenFor(t, auth.PermAuditsRead)); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without cluster:read, got %d", rec.Code)
	}
}

func TestClusterStatusReportsSingleNodeLeader(t *testing.T) {
	mux := newTestMux(t)

	rec := getClusterStatus(t, mux, tokenFor(t, auth.PermClusterRead))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		LeaderID      string `json:"leaderId"`
		LeaderAddress string `json:"leaderAddress"`
		Nodes         []struct {
			ID       string          `json:"id"`
			Address  string          `json:"address"`
			Suffrage string          `json:"suffrage"`
			Leader   bool            `json:"leader"`
			Lag      json.RawMessage `json:"lag"`
		} `json:"nodes"`
		CommitIndex  *uint64 `json:"commitIndex"`
		LastLogIndex *uint64 `json:"lastLogIndex"`
		Term         *uint64 `json:"term"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.LeaderID != "middleware-test" || body.LeaderAddress == "" {
		t.Fatalf("unexpected leader %q at %q", body.LeaderID, body.LeaderAddress)
	}
	if len(body.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %+v", body.Nodes)
	}
	n := body.Nodes[0]
	if n.ID != body.LeaderID || n.Address != body.LeaderAddress || n.Suffrage != "Voter" || !n.Leader {
		t.Fatalf("unexpected node %+v", n)
	}
	if string(n.Lag) != "null" {
		t.Fatalf("expected lag to be null until replication lag is tracked, got %s", n.Lag)
	}
	if body.CommitIndex == nil || body.LastLogIndex == nil || body.Term == nil {
		t.Fatalf("expected commitIndex, lastLogIndex and term in %s", rec.Body.String())
	}
	if *body.Term < 1 || *body.LastLogIndex < 1 || *body.CommitIndex > *body.LastLogIndex {
		t.Fatalf("implausible raft indexes: commit=%d last=%d term=%d", *body.CommitIndex, *body.LastLogIndex, *body.Term)
	}
}
