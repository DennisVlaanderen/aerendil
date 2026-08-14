package store

import (
	"errors"
	"fmt"
	"sort"
)

// ErrLastEnvironment is returned when deleting the target environment
// would leave zero environments -- an invariant on count, not any single
// environment being protected (contrast ErrProtectedSystemGroup).
var ErrLastEnvironment = errors.New("cannot delete the last remaining environment")

// ErrEnvironmentHasFlags is returned when the target environment still has
// a flag scoped to it -- FSM-enforced since deleting it would otherwise
// silently orphan those flags.
var ErrEnvironmentHasFlags = errors.New("cannot delete an environment that still has flags")

// ErrEnvironmentHasCredentials is returned when an application credential
// is still scoped to the target environment -- same referential-integrity
// tier as ErrEnvironmentHasFlags.
var ErrEnvironmentHasCredentials = errors.New("cannot delete an environment that still has application credentials")

// Environment is a named deployment target (e.g. "Production", "Staging")
// flags and group permissions are scoped to. Order is stamped at creation
// from the current count and is otherwise immutable for now.
type Environment struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Order   int    `json:"order"`
	Version uint64 `json:"version"`
}

func (f *fsm) applyEnvironment(index uint64, cmd command) any {
	switch cmd.Op {
	case opSet:
		cmd.Environment.Version = index
		f.environments[cmd.Environment.ID] = *cmd.Environment
		return *cmd.Environment
	case opDelete:
		if _, ok := f.environments[cmd.Key]; ok && len(f.environments) <= 1 {
			return fmt.Errorf("%w: %q", ErrLastEnvironment, cmd.Key)
		}
		if f.hasFlagsInEnvironmentLocked(cmd.Key) {
			return fmt.Errorf("%w: %q", ErrEnvironmentHasFlags, cmd.Key)
		}
		if f.hasCredentialsInEnvironmentLocked(cmd.Key) {
			return fmt.Errorf("%w: %q", ErrEnvironmentHasCredentials, cmd.Key)
		}
		delete(f.environments, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

func (f *fsm) getEnvironment(id string) (Environment, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.environments[id]
	return e, ok
}

// listEnvironments returns every environment ordered by Order ascending,
// ID as a tiebreaker -- map iteration order is otherwise randomized.
func (f *fsm) listEnvironments() []Environment {
	f.mu.RLock()
	defer f.mu.RUnlock()
	envs := make([]Environment, 0, len(f.environments))
	for _, e := range f.environments {
		envs = append(envs, e)
	}
	sort.Slice(envs, func(i, j int) bool {
		if envs[i].Order != envs[j].Order {
			return envs[i].Order < envs[j].Order
		}
		return envs[i].ID < envs[j].ID
	})
	return envs
}

// EnvironmentRepository provides environment operations against the store.
// Obtain one via Store.Environments().
type EnvironmentRepository struct {
	store *Store
}

// Get returns the current state of an environment, if it exists.
func (r EnvironmentRepository) Get(id string) (Environment, bool) {
	return r.store.fsm.getEnvironment(id)
}

// List returns all known environments, ordered by Order ascending.
func (r EnvironmentRepository) List() []Environment {
	return r.store.fsm.listEnvironments()
}

// Set applies an environment create/update through Raft consensus.
func (r EnvironmentRepository) Set(env Environment) (Environment, error) {
	resp, err := r.store.apply(command{Op: opSet, Entity: entityEnvironment, Environment: &env})
	if err != nil {
		return Environment{}, err
	}
	switch v := resp.(type) {
	case Environment:
		return v, nil
	case error:
		return Environment{}, v
	default:
		return Environment{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// Delete removes an environment by ID. Fails with ErrLastEnvironment,
// ErrEnvironmentHasFlags, or ErrEnvironmentHasCredentials -- fast
// pre-checks here, fsm.applyEnvironment is the ultimate source of truth.
func (r EnvironmentRepository) Delete(id string) error {
	if _, ok := r.store.fsm.getEnvironment(id); ok && len(r.store.fsm.listEnvironments()) <= 1 {
		return fmt.Errorf("%w: %q", ErrLastEnvironment, id)
	}
	if r.store.fsm.hasFlagsInEnvironment(id) {
		return fmt.Errorf("%w: %q", ErrEnvironmentHasFlags, id)
	}
	if r.store.fsm.hasCredentialsInEnvironment(id) {
		return fmt.Errorf("%w: %q", ErrEnvironmentHasCredentials, id)
	}

	resp, err := r.store.apply(command{Op: opDelete, Entity: entityEnvironment, Key: id})
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}
