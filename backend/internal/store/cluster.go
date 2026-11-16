package store

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// newRaft wires up a Raft node backed by BoltDB (log/stable store) and a
// file snapshot store, bootstrapping a single-member cluster on first run
// when bootstrap is true; restarts rejoin existing state instead.
//
// The open BoltDB store is returned so the caller can close it: Raft shuts
// down its own goroutines but never closes the LogStore/StableStore it was
// handed, so whoever opened raft.db owns releasing it.
func newRaft(nodeID, bindAddr, dataDir string, bootstrap bool) (*raft.Raft, *fsm, io.Closer, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, nil, fmt.Errorf("create raft data dir: %w", err)
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	fsmStore := newFSM()

	boltStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft.db"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open raft log store: %w", err)
	}
	// Every return past this point owns boltStore. Close it on any failure;
	// hand it to the caller only on the success path.
	ok := false
	defer func() {
		if !ok {
			_ = boltStore.Close()
		}
	}()

	snapshotStore, err := raft.NewFileSnapshotStore(dataDir, 2, os.Stderr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open raft snapshot store: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve raft bind address: %w", err)
	}
	transport, err := raft.NewTCPTransport(bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open raft transport: %w", err)
	}

	r, err := raft.NewRaft(config, fsmStore, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("start raft node: %w", err)
	}

	if bootstrap {
		hasState, err := raft.HasExistingState(boltStore, boltStore, snapshotStore)
		if err != nil {
			_ = r.Shutdown().Error()
			return nil, nil, nil, fmt.Errorf("check existing raft state: %w", err)
		}
		if !hasState {
			bootstrapConfig := raft.Configuration{
				Servers: []raft.Server{
					{ID: config.LocalID, Address: transport.LocalAddr()},
				},
			}
			if err := r.BootstrapCluster(bootstrapConfig).Error(); err != nil {
				_ = r.Shutdown().Error()
				return nil, nil, nil, fmt.Errorf("bootstrap raft cluster: %w", err)
			}
		}
	}

	ok = true
	return r, fsmStore, boltStore, nil
}

// ClusterStatus is a point-in-time view of the local node's Raft state.
type ClusterStatus struct {
	LeaderID      string     `json:"leaderId"`
	LeaderAddress string     `json:"leaderAddress"`
	Nodes         []NodeInfo `json:"nodes"`
	CommitIndex   uint64     `json:"commitIndex"`
	LastLogIndex  uint64     `json:"lastLogIndex"`
	Term          uint64     `json:"term"`
}

// NodeInfo is one server in the Raft configuration.
type NodeInfo struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Suffrage string `json:"suffrage"`
	Leader   bool   `json:"leader"`
	// Lag is always nil: hashicorp/raft exposes no per-follower replication
	// lag. Kept in the shape so adding it later is not a breaking change.
	Lag *uint64 `json:"lag"`
}

// ClusterStatus reports the leader, the configured servers, and the local
// node's commit index, last log index, and term.
func (s *Store) ClusterStatus() (ClusterStatus, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return ClusterStatus{}, fmt.Errorf("get raft configuration: %w", err)
	}
	leaderAddr, leaderID := s.raft.LeaderWithID()

	servers := future.Configuration().Servers
	nodes := make([]NodeInfo, len(servers))
	for i, srv := range servers {
		nodes[i] = NodeInfo{
			ID:       string(srv.ID),
			Address:  string(srv.Address),
			Suffrage: srv.Suffrage.String(),
			Leader:   srv.ID == leaderID,
		}
	}

	stats := s.raft.Stats()
	status := ClusterStatus{LeaderID: string(leaderID), LeaderAddress: string(leaderAddr), Nodes: nodes}
	for key, dst := range map[string]*uint64{
		"commit_index":   &status.CommitIndex,
		"last_log_index": &status.LastLogIndex,
		"term":           &status.Term,
	} {
		v, err := strconv.ParseUint(stats[key], 10, 64)
		if err != nil {
			return ClusterStatus{}, fmt.Errorf("parse raft stat %s: %w", key, err)
		}
		*dst = v
	}
	return status, nil
}
