package store

import (
	"errors"
	"fmt"
	"sort"
)

// ErrProtectedSystemGroup is returned when targeting a System group
// (only Admin), so the API can 403 without its own AdminGroupID checks.
var ErrProtectedSystemGroup = errors.New("this group is a protected system group and cannot be modified or deleted")

// Group is a named set of permissions assigned to users.
//
// EnvironmentIDs grants access to those environments (empty means none). It
// is API-validated only; a dangling ID after an environment is deleted is
// harmless.
type Group struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Permissions    []string `json:"permissions,omitempty"`
	EnvironmentIDs []string `json:"environmentIds,omitempty"`
	System         bool     `json:"system"`
	Version        uint64   `json:"version"`
}

// AdminGroupID is the Admin group's fixed ID.
const AdminGroupID = "admin"

func (f *fsm) applyGroup(index uint64, cmd command) any {
	switch cmd.Op {
	case opSet:
		if existing, ok := f.groups[cmd.Group.ID]; ok && existing.System {
			return fmt.Errorf("%w: %q", ErrProtectedSystemGroup, existing.ID)
		}
		cmd.Group.Version = index
		f.groups[cmd.Group.ID] = *cmd.Group
		return *cmd.Group
	case opDelete:
		if existing, ok := f.groups[cmd.Key]; ok && existing.System {
			return fmt.Errorf("%w: %q", ErrProtectedSystemGroup, existing.ID)
		}
		delete(f.groups, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

func (f *fsm) getGroup(id string) (Group, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	g, ok := f.groups[id]
	return g, ok
}

func (f *fsm) listGroups() []Group {
	f.mu.RLock()
	defer f.mu.RUnlock()
	groups := make([]Group, 0, len(f.groups))
	for _, g := range f.groups {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	return groups
}

// GroupRepository provides group operations; get one via Store.Groups().
type GroupRepository struct {
	store *Store
}

// Get returns a group, if it exists.
func (r GroupRepository) Get(id string) (Group, bool) {
	return r.store.fsm.getGroup(id)
}

// List returns all known groups, ordered by ID.
func (r GroupRepository) List() []Group {
	return r.store.fsm.listGroups()
}

// Set creates or updates a group through Raft. The Admin group is rejected
// here as a fast pre-check; fsm.Apply enforces it too.
func (r GroupRepository) Set(group Group) (Group, error) {
	if existing, ok := r.store.fsm.getGroup(group.ID); ok && existing.System {
		return Group{}, fmt.Errorf("%w: %q", ErrProtectedSystemGroup, existing.ID)
	}

	resp, err := r.store.apply(command{Op: opSet, Entity: entityGroup, Group: &group})
	if err != nil {
		return Group{}, err
	}
	switch v := resp.(type) {
	case Group:
		return v, nil
	case error:
		return Group{}, v
	default:
		return Group{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// Delete removes a group by ID. The Admin group can never be deleted.
func (r GroupRepository) Delete(id string) error {
	if existing, ok := r.store.fsm.getGroup(id); ok && existing.System {
		return fmt.Errorf("%w: %q", ErrProtectedSystemGroup, existing.ID)
	}

	resp, err := r.store.apply(command{Op: opDelete, Entity: entityGroup, Key: id})
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}
