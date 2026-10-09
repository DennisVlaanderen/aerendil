package store

import (
	"errors"
	"fmt"
	"sort"
)

// ErrUnknownEnvironment means a flag's EnvironmentID names no Environment.
var ErrUnknownEnvironment = errors.New("environment does not exist")

// Flag is a feature flag in one Environment; the same Key in another
// environment is a separate record (see flagMapKey).
type Flag struct {
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Enabled       bool   `json:"enabled"`
	Value         string `json:"value,omitempty"`
	Version       uint64 `json:"version"`
}

// flagMapKey is the internal fsm.flags key for an environment+key pair.
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
		// Validate all environments first so a bad ID rejects the whole batch.
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
	sort.Slice(flags, func(i, j int) bool { return flags[i].Key < flags[j].Key })
	return flags
}

// hasFlagsInEnvironmentLocked reports whether environmentID has flags.
// Caller holds f.mu.
func (f *fsm) hasFlagsInEnvironmentLocked(environmentID string) bool {
	for _, flag := range f.flags {
		if flag.EnvironmentID == environmentID {
			return true
		}
	}
	return false
}

// hasFlagsInEnvironment is hasFlagsInEnvironmentLocked with a read lock.
func (f *fsm) hasFlagsInEnvironment(environmentID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.hasFlagsInEnvironmentLocked(environmentID)
}

// FlagRepository provides flag operations; get one via Store.Flags().
type FlagRepository struct {
	store *Store
}

// Get returns a flag in an environment, if it exists.
func (r FlagRepository) Get(environmentID, key string) (Flag, bool) {
	return r.store.fsm.getFlag(environmentID, key)
}

// List returns all flags scoped to environmentID, ordered by Key.
func (r FlagRepository) List(environmentID string) []Flag {
	return r.store.fsm.listFlags(environmentID)
}

// Set applies a flag change through Raft; an unknown EnvironmentID returns
// ErrUnknownEnvironment.
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

// SetMany writes one key/enabled/value to every listed environment
// atomically (ErrUnknownEnvironment if any ID is unknown). Discrete fields,
// not []Flag, so values can't diverge per environment.
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

// Delete removes a flag from an environment through Raft.
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
