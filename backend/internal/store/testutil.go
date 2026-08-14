package store

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// NewTestStore opens a real, single-node Store on a temp data dir and
// waits for it to become Raft leader, so other packages' tests can spin
// up a live store without duplicating Raft bootstrap boilerplate.
func NewTestStore(t testing.TB) *Store {
	t.Helper()

	dataDir, err := os.MkdirTemp("", "aerendil-test-store-*")
	if err != nil {
		t.Fatalf("create temp data dir: %v", err)
	}
	// Store.Close doesn't release the BoltDB file handle, so on Windows
	// t.TempDir()-style cleanup can race an open lock. Best-effort removal
	// avoids failing tests over it.
	t.Cleanup(func() {
		_ = os.RemoveAll(dataDir)
	})

	s, err := Open(Config{
		NodeID:    "test-node",
		BindAddr:  "127.0.0.1:0",
		DataDir:   dataDir,
		Bootstrap: true,
	})
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s.raft.State() == raft.Leader {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for raft node to become leader")
	return nil
}
