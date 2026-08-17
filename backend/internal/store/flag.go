package store

import (
	"errors"
	"fmt"
)

// ErrUnknownEnvironment is returned when a flag's EnvironmentID doesn't
// match any existing Environment.
var ErrUnknownEnvironment = errors.New("environment does not exist")

// Flag is a feature flag scoped to exactly one Environment. The same Key
// can exist independently in multiple environments, each a distinct
// record (see flagMapKey) rather than one record with per-env overrides.
type Flag struct {
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Enabled       bool   `json:"enabled"`
	Value         string `json:"value,omitempty"`
	Version       uint64 `json:"version"`
}

// flagMapKey is the fsm.flags map key for an environment+key pair -- an
// internal detail; FlagRepository always takes/returns them separately.
func flagMapKey(environmentID, key string) string {
	return environmentID + "/" + key
}

func (f *fsm) applyFlag(index uint64, cmd command) any {
	switch cmd.Op {
	case opSet:
		if _, ok := f.environments[cmd.Flag.EnvironmentID]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownEnvironment, cmd.Flag.EnvironmentID)
		}
		cmd.Flag.Version = index
		f.flags[flagMapKey(cmd.Flag.EnvironmentID, cmd.Flag.Key)] = *cmd.Flag
		return *cmd.Flag
	case opSetBatch:
		// Validate every target environment before writing any -- one Raft
		// log entry under one lock, so a bad ID rejects the whole batch atomically.
		for _, flag := range cmd.Flags {
			if _, ok := f.environments[flag.EnvironmentID]; !ok {
				return fmt.Errorf("%w: %q", ErrUnknownEnvironment, flag.EnvironmentID)
			}
		}
		applied := make([]Flag, len(cmd.Flags))
		for i, flag := range cmd.Flags {
			flag.Version = index
			f.flags[flagMapKey(flag.EnvironmentID, flag.Key)] = flag
			applied[i] = flag
		}
		return applied
	case opDelete:
		delete(f.flags, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

func (f *fsm) getFlag(environmentID, key string) (Flag, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	flag, ok := f.flags[flagMapKey(environmentID, key)]
	return flag, ok
}

func (f *fsm) listFlags(environmentID string) []Flag {
	f.mu.RLock()
	defer f.mu.RUnlock()
	flags := make([]Flag, 0)
	for _, flag := range f.flags {
		if flag.EnvironmentID == environmentID {
			flags = append(flags, flag)
		}
	}
	return flags
}

// hasFlagsInEnvironmentLocked reports whether any flag is scoped to
// environmentID. Caller must already hold f.mu.
func (f *fsm) hasFlagsInEnvironmentLocked(environmentID string) bool {
	for _, flag := range f.flags {
		if flag.EnvironmentID == environmentID {
			return true
		}
	}
	return false
}

// hasFlagsInEnvironment is the read-locking counterpart of
// hasFlagsInEnvironmentLocked, for use outside of Apply.
func (f *fsm) hasFlagsInEnvironment(environmentID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.hasFlagsInEnvironmentLocked(environmentID)
}

// FlagRepository provides flag operations against the store. Obtain one
// via Store.Flags().
type FlagRepository struct {
	store *Store
}

// Get returns the current value of a flag in a specific environment, if it
// exists.
func (r FlagRepository) Get(environmentID, key string) (Flag, bool) {
	return r.store.fsm.getFlag(environmentID, key)
}

// List returns all flags scoped to environmentID.
func (r FlagRepository) List(environmentID string) []Flag {
	return r.store.fsm.listFlags(environmentID)
}

// Set applies a single flag change through Raft consensus.
// flag.EnvironmentID must reference an existing Environment
// (ErrUnknownEnvironment otherwise).
func (r FlagRepository) Set(flag Flag) (Flag, error) {
	resp, err := r.store.apply(command{Op: opSet, Entity: entityFlag, Flag: &flag})
	if err != nil {
		return Flag{}, err
	}
	switch v := resp.(type) {
	case Flag:
		return v, nil
	case error:
		return Flag{}, v
	default:
		return Flag{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// SetMany writes the same key/enabled/value into every listed environment
// as one atomic Raft command -- all succeed or none do
// (ErrUnknownEnvironment if any ID doesn't exist). Takes discrete fields
// rather than []Flag so callers can't submit divergent values per
// environment for what should be one consistent write.
func (r FlagRepository) SetMany(key string, enabled bool, value string, environmentIDs []string) ([]Flag, error) {
	flags := make([]Flag, len(environmentIDs))
	for i, envID := range environmentIDs {
		flags[i] = Flag{EnvironmentID: envID, Key: key, Enabled: enabled, Value: value}
	}

	resp, err := r.store.apply(command{Op: opSetBatch, Entity: entityFlag, Flags: flags})
	if err != nil {
		return nil, err
	}
	switch v := resp.(type) {
	case []Flag:
		return v, nil
	case error:
		return nil, v
	default:
		return nil, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// Delete removes a flag from a specific environment through Raft consensus.
func (r FlagRepository) Delete(environmentID, key string) error {
	resp, err := r.store.apply(command{Op: opDelete, Entity: entityFlag, Key: flagMapKey(environmentID, key)})
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}
