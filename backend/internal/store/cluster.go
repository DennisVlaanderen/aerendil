package store

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// newRaft wires up a Raft node backed by BoltDB (log/stable store) and a
// file snapshot store, bootstrapping a single-member cluster on first run
// when bootstrap is true; restarts rejoin existing state instead.
func newRaft(nodeID, bindAddr, dataDir string, bootstrap bool) (*raft.Raft, *fsm, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create raft data dir: %w", err)
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	fsmStore := newFSM()

	boltStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("open raft log store: %w", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(dataDir, 2, os.Stderr)
	if err != nil {
		return nil, nil, fmt.Errorf("open raft snapshot store: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve raft bind address: %w", err)
	}
	transport, err := raft.NewTCPTransport(bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, nil, fmt.Errorf("open raft transport: %w", err)
	}

	r, err := raft.NewRaft(config, fsmStore, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		return nil, nil, fmt.Errorf("start raft node: %w", err)
	}

	if bootstrap {
		hasState, err := raft.HasExistingState(boltStore, boltStore, snapshotStore)
		if err != nil {
			return nil, nil, fmt.Errorf("check existing raft state: %w", err)
		}
		if !hasState {
			bootstrapConfig := raft.Configuration{
				Servers: []raft.Server{
					{ID: config.LocalID, Address: transport.LocalAddr()},
				},
			}
			if err := r.BootstrapCluster(bootstrapConfig).Error(); err != nil {
				return nil, nil, fmt.Errorf("bootstrap raft cluster: %w", err)
			}
		}
	}

	return r, fsmStore, nil
}
