package store

import (
	"errors"
	"fmt"
)

// ErrUnknownEnvironment is returned by FlagRepository.Set/SetMany (and,
// ultimately, fsm.applyFlag) when the flag's EnvironmentID doesn't match any
// existing Environment. errors.Is-comparable, mirroring the other FSM-level
// sentinels (ErrLastAdmin, ErrProtectedSystemGroup, ErrLastEnvironment).
var ErrUnknownEnvironment = errors.New("environment does not exist")

// Flag is a single feature flag record replicated across the cluster,
// scoped to exactly one Environment. The same Key can exist independently
// in multiple environments -- each is a distinct Flag record with its own
// Enabled/Value/Version, isolated via the FSM's composite map key (see
// flagMapKey), not a shared record with per-environment overrides.
type Flag struct {
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Enabled       bool   `json:"enabled"`
	Value         string `json:"value,omitempty"`
	Version       uint64 `json:"version"`
}

// flagMapKey is the fsm.flags map key for a given environment+key pair --
// an internal detail callers never see; FlagRepository always takes/returns
// EnvironmentID and Key as separate fields.
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
		// Validate every target environment before writing any of them --
		// one Raft log entry, one f.mu.Lock() for the whole dispatch (see
		// fsm.Apply), so this is genuinely atomic: a bad ID anywhere in the
		// batch rejects the whole command with zero partial writes.
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

// hasFlagsInEnvironmentLocked reports whether any flag is currently scoped
// to environmentID -- called directly by applyEnvironment's opDelete case,
// which already holds f.mu (via fsm.Apply's single lock for the whole
// dispatch), so it must not try to acquire it again. Caller must already
// hold f.mu.
func (f *fsm) hasFlagsInEnvironmentLocked(environmentID string) bool {
	for _, flag := range f.flags {
		if flag.EnvironmentID == environmentID {
			return true
		}
	}
	return false
}

// hasFlagsInEnvironment is the read-locking counterpart of
// hasFlagsInEnvironmentLocked, for use outside of Apply (e.g. a
// repository's fast pre-check before proposing a command to Raft at all) --
// mirrors isSoleActiveAdmin/isSoleActiveAdminLocked's split in user.go.
func (f *fsm) hasFlagsInEnvironment(environmentID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.hasFlagsInEnvironmentLocked(environmentID)
}

// FlagRepository provides flag operations against the store. Obtain one via
// Store.Flags(); it's a stateless one-field wrapper, cheap to construct on
// every call, so it never needs to be cached or stored as a field.
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

// Set applies a single flag change through Raft consensus. flag.EnvironmentID
// must reference an existing Environment (ErrUnknownEnvironment otherwise).
// It only succeeds on the cluster leader; with a single bootstrapped node
// that is always the case.
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

// SetMany writes the same key/enabled/value into every listed environment as
// a single atomic Raft command -- all environments succeed together, or none
// of them are written (ErrUnknownEnvironment if any ID doesn't exist). This
// is deliberately the only way to write a flag into multiple environments at
// once: taking (key, enabled, value, ids) rather than a raw []Flag closes
// off the possibility of an API bug submitting divergent values per
// environment for what's supposed to be one consistent multi-environment
// create.
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
