package store

import (
	"errors"
	"fmt"
	"sort"
)

// ErrLastEnvironment is returned by EnvironmentRepository.Delete (and,
// ultimately, fsm.applyEnvironment) when deleting the target environment
// would leave zero environments. Mirrors ErrLastAdmin's pattern -- the
// invariant is about the *count*, not any single environment being
// protected (contrast with ErrProtectedSystemGroup).
var ErrLastEnvironment = errors.New("cannot delete the last remaining environment")

// ErrEnvironmentHasFlags is returned by EnvironmentRepository.Delete (and,
// ultimately, fsm.applyEnvironment) when the target environment still has
// at least one flag scoped to it. This referential integrity check is
// FSM-enforced, the same tier as ErrLastAdmin/ErrProtectedSystemGroup/
// ErrLastEnvironment, since a deleted environment would otherwise silently
// orphan any flags still pointing at it -- real data loss, not a harmless
// dangling reference.
var ErrEnvironmentHasFlags = errors.New("cannot delete an environment that still has flags")

// ErrEnvironmentHasCredentials is returned by EnvironmentRepository.Delete
// (and, ultimately, fsm.applyEnvironment) when an application credential is
// still scoped to the target environment -- same referential-integrity tier
// as ErrEnvironmentHasFlags, since deleting the environment would otherwise
// leave the credential pointing at nothing.
var ErrEnvironmentHasCredentials = errors.New("cannot delete an environment that still has application credentials")

// Environment is a named deployment target (e.g. "Production", "Staging")
// that flags and group permissions will later be scoped to. Order is
// stamped at creation time from the current environment count and is
// otherwise immutable -- reordering/promotion direction is a follow-up
// once environment-scoped flags exist to make ordering matter.
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

// listEnvironments returns every environment ordered by Order ascending
// (ID as a tiebreaker for determinism) -- map iteration order is
// randomized, and callers need a stable, meaningful order back from
// List() itself rather than having to sort client-side.
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

// Delete removes an environment by ID. Fails with ErrLastEnvironment if it
// is the only one remaining, or ErrEnvironmentHasFlags if any flag is still
// scoped to it -- both are fast pre-checks here, with fsm.applyEnvironment
// as the ultimate source of truth for the same rules (mirrors
// GroupRepository.Delete's pre-check + FSM-enforcement pattern).
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
