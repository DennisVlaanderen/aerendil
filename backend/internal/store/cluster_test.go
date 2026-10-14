package store

import (
	"testing"
	"time"
)

// TestReopenAfterCloseDoesNotBlockOnTheBoltLock guards the lifecycle of the
// raft.db BoltDB handle: newRaft opens it, so Store.Close must release it.
// bbolt takes an exclusive file lock on Open and waits while it is held, so
// if Close leaves the handle open a second Open on the same data dir hangs.
func TestReopenAfterCloseDoesNotBlockOnTheBoltLock(t *testing.T) {
	dataDir := t.TempDir()
	cfg := Config{
		NodeID:    "reopen-node",
		BindAddr:  "127.0.0.1:0",
		DataDir:   dataDir,
		Bootstrap: true,
	}

	first, err := Open(cfg)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen in the background so a leaked lock surfaces as a timeout rather
	// than hanging the whole test binary.
	done := make(chan error, 1)
	go func() {
		second, err := Open(cfg)
		if err == nil {
			_ = second.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second Open after Close: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("second Open blocked on the raft.db lock: Store.Close did not release the BoltDB handle")
	}
}
