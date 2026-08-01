package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/raft"
)

// ErrNotLeader is returned (wrapped) by any mutation when the local node
// isn't the current Raft leader. Callers that want to tolerate this (e.g.
// bootstrap seeding on a follower) can check it with errors.Is.
var ErrNotLeader = errors.New("not the raft leader")

// Config describes how to open or bootstrap a Raft-backed Store.
type Config struct {
	NodeID    string
	BindAddr  string
	DataDir   string
	Bootstrap bool
}

// Store is an embedded, Raft-replicated key/value store for feature flags,
// users, groups, and environments. Entity-specific operations are grouped
// into repositories (Flags/Users/Groups/Environments) rather than living
// directly on Store, so adding a new entity in the future means adding a
// repository, not growing this type.
type Store struct {
	raft *raft.Raft
	fsm  *fsm
}

// Open starts (or rejoins) a Raft node and returns a ready-to-use Store.
func Open(cfg Config) (*Store, error) {
	r, fsmStore, err := newRaft(cfg.NodeID, cfg.BindAddr, cfg.DataDir, cfg.Bootstrap)
	if err != nil {
		return nil, err
	}
	return &Store{raft: r, fsm: fsmStore}, nil
}

// Flags returns a repository for flag operations against the store.
func (s *Store) Flags() FlagRepository { return FlagRepository{store: s} }

// Users returns a repository for user operations against the store.
func (s *Store) Users() UserRepository { return UserRepository{store: s} }

// Groups returns a repository for group operations against the store.
func (s *Store) Groups() GroupRepository { return GroupRepository{store: s} }

// Audits returns a repository for audit-log operations against the store.
func (s *Store) Audits() AuditRepository { return AuditRepository{store: s} }

// Environments returns a repository for environment operations against the store.
func (s *Store) Environments() EnvironmentRepository { return EnvironmentRepository{store: s} }

// apply centralizes the boilerplate every mutating repository method needs:
// confirm this node is the Raft leader, marshal the command, submit it via
// raft.Apply, and surface any submission-level error. The caller still
// type-asserts/switches on the returned response, since each entity command
// can succeed with a different concrete type (Flag/User/Group) or fail with
// an fsm-level business-rule error (e.g. ErrUsernameTaken) instead of a
// submission error.
func (s *Store) apply(cmd command) (any, error) {
	if s.raft.State() != raft.Leader {
		return nil, fmt.Errorf("%w (leader is %q)", ErrNotLeader, s.raft.Leader())
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("encode command: %w", err)
	}

	future := s.raft.Apply(data, 5*time.Second)
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("apply command: %w", err)
	}
	return future.Response(), nil
}

// Close shuts down the Raft node.
func (s *Store) Close() error {
	return s.raft.Shutdown().Error()
}
