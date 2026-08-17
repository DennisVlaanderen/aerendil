// Package store is Aerendil's Raft-replicated data layer: an in-memory FSM
// for flags, users, groups, audits, and environments, kept consistent via
// hashicorp/raft. See Store.
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

// Store is an embedded, Raft-replicated key/value store for flags, users,
// groups, and environments. Operations are grouped into per-entity
// repositories (Flags/Users/...) rather than living on Store directly.
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

// ApplicationCredentials returns a repository for application-credential
// operations against the store.
func (s *Store) ApplicationCredentials() ApplicationCredentialRepository {
	return ApplicationCredentialRepository{store: s}
}

// apply centralizes the boilerplate every mutating repository method
// needs: confirm leadership, marshal the command, submit via raft.Apply.
// Callers still type-switch the response, since each entity command
// returns a different concrete type or an fsm-level business error.
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
