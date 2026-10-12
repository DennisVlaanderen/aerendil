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
	Color   string `json:"color"`
}

func (f *fsm) applyEnvironment(logIndex uint64, raftCommand command) any {
	switch raftCommand.Op {
	case opSet:
		raftCommand.Environment.Version = logIndex
		f.environments[raftCommand.Environment.ID] = *raftCommand.Environment
		return *raftCommand.Environment
	case opDelete:
		environmentID := raftCommand.Key
		if _, exists := f.environments[environmentID]; exists && len(f.environments) <= 1 {
			return fmt.Errorf("%w: %q", ErrLastEnvironment, environmentID)
		}
		if f.hasFlagsInEnvironmentLocked(environmentID) {
			return fmt.Errorf("%w: %q", ErrEnvironmentHasFlags, environmentID)
		}
		if f.hasCredentialsInEnvironmentLocked(environmentID) {
			return fmt.Errorf("%w: %q", ErrEnvironmentHasCredentials, environmentID)
		}
		delete(f.environments, environmentID)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", raftCommand.Op)
	}
}

func (f *fsm) getEnvironment(id string) (Environment, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	environment, exists := f.environments[id]
	return environment, exists
}

// listEnvironments returns every environment ordered by Order ascending,
// ID as a tiebreaker -- map iteration order is otherwise randomized.
func (f *fsm) listEnvironments() []Environment {
	f.mu.RLock()
	defer f.mu.RUnlock()
	sortedEnvironments := make([]Environment, 0, len(f.environments))
	for _, environment := range f.environments {
		sortedEnvironments = append(sortedEnvironments, environment)
	}
	sort.Slice(sortedEnvironments, func(left, right int) bool {
		leftEnvironment, rightEnvironment := sortedEnvironments[left], sortedEnvironments[right]
		if leftEnvironment.Order != rightEnvironment.Order {
			return leftEnvironment.Order < rightEnvironment.Order
		}
		return leftEnvironment.ID < rightEnvironment.ID
	})
	return sortedEnvironments
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
func (r EnvironmentRepository) Set(environment Environment) (Environment, error) {
	applyResponse, applyErr := r.store.apply(command{Op: opSet, Entity: entityEnvironment, Environment: &environment})
	if applyErr != nil {
		return Environment{}, applyErr
	}
	switch result := applyResponse.(type) {
	case Environment:
		return result, nil
	case error:
		return Environment{}, result
	default:
		return Environment{}, fmt.Errorf("unexpected apply response type %T", applyResponse)
	}
}

// Delete removes an environment by ID. Fails with ErrLastEnvironment,
// ErrEnvironmentHasFlags, or ErrEnvironmentHasCredentials -- fast
// pre-checks here, fsm.applyEnvironment is the ultimate source of truth.
func (r EnvironmentRepository) Delete(id string) error {
	if _, exists := r.store.fsm.getEnvironment(id); exists && len(r.store.fsm.listEnvironments()) <= 1 {
		return fmt.Errorf("%w: %q", ErrLastEnvironment, id)
	}
	if r.store.fsm.hasFlagsInEnvironment(id) {
		return fmt.Errorf("%w: %q", ErrEnvironmentHasFlags, id)
	}
	if r.store.fsm.hasCredentialsInEnvironment(id) {
		return fmt.Errorf("%w: %q", ErrEnvironmentHasCredentials, id)
	}

	applyResponse, applyErr := r.store.apply(command{Op: opDelete, Entity: entityEnvironment, Key: id})
	if applyErr != nil {
		return applyErr
	}
	if fsmErr, isError := applyResponse.(error); isError {
		return fsmErr
	}
	return nil
}
