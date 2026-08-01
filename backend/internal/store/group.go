package store

import (
	"errors"
	"fmt"
)

// ErrProtectedSystemGroup is returned by GroupRepository.Set/Delete (and,
// ultimately, fsm.applyGroup) when the target group has System set --
// currently only the Admin group. The single sentinel is what lets api's
// writeStoreError map this to a 403 without every caller needing its own
// pre-check keyed on AdminGroupID; errors.Is-comparable like
// ErrUsernameTaken/ErrLastAdmin.
var ErrProtectedSystemGroup = errors.New("this group is a protected system group and cannot be modified or deleted")

// Group is a named set of permissions that can be assigned to users.
//
// AdminGroupID is the fixed, well-known ID of the group seeded at
// bootstrap. It is immutable (System is true) and its bypass of every
// permission check is anchored on this ID, not on Name or on an explicit
// "*" permission entry -- see auth.Service.Resolve.
//
// EnvironmentIDs grants this group's members access to specific
// environments' flags; an empty/nil list means no environment access at
// all (deny-by-default, mirroring how an empty Permissions list already
// means zero permissions) -- Admin's unconditional bypass covers
// environments the same way it covers every other permission, so this
// field is never consulted for the Admin group. Unlike Permissions
// (validated against auth.IsKnownPermission in api/groups.go),
// EnvironmentIDs is API-validated only, not FSM-enforced: a dangling ID
// left behind after an environment is deleted is harmless (it grants
// access to nothing), unlike a Flag pointing at a deleted environment,
// which would be silent data loss -- see store.ErrEnvironmentHasFlags.
type Group struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Permissions    []string `json:"permissions,omitempty"`
	EnvironmentIDs []string `json:"environmentIds,omitempty"`
	System         bool     `json:"system"`
	Version        uint64   `json:"version"`
}

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
	return groups
}

// GroupRepository provides group operations against the store. Obtain one
// via Store.Groups().
type GroupRepository struct {
	store *Store
}

// Get returns the current state of a group, if it exists.
func (r GroupRepository) Get(id string) (Group, bool) {
	return r.store.fsm.getGroup(id)
}

// List returns all known groups.
func (r GroupRepository) List() []Group {
	return r.store.fsm.listGroups()
}

// Set applies a group create/update through Raft consensus. The Admin
// group is rejected before it's even proposed to Raft as a fast path;
// fsm.Apply enforces the same rule as the ultimate source of truth.
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
