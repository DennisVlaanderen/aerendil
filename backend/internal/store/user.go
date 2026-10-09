package store

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

// ErrUsernameTaken means another user has that username.
var ErrUsernameTaken = errors.New("username is already taken")

// ErrLastAdmin is returned when a delete, deactivation or Admin-membership
// removal would leave no active admin.
var ErrLastAdmin = errors.New("cannot remove the last remaining admin account")

// User is a persisted account. Password handling lives in auth; store only
// enforces invariants like username uniqueness.
type User struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	PasswordHash []byte   `json:"password_hash,omitempty"`
	GroupIDs     []string `json:"group_ids,omitempty"`
	Active       bool     `json:"active"`
	Version      uint64   `json:"version"`
}

func (f *fsm) applyUser(index uint64, cmd command) interface{} {
	switch cmd.Op {
	case opSet:
		for id, existing := range f.users {
			if id != cmd.User.ID && existing.Username == cmd.User.Username {
				return ErrUsernameTaken
			}
		}
		if f.isSoleActiveAdminLocked(cmd.User.ID) && !isActiveAdmin(*cmd.User) {
			return ErrLastAdmin
		}
		cmd.User.Version = index
		f.users[cmd.User.ID] = *cmd.User
		return *cmd.User
	case opDelete:
		if f.isSoleActiveAdminLocked(cmd.Key) {
			return ErrLastAdmin
		}
		delete(f.users, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

func isActiveAdmin(u User) bool {
	return u.Active && slices.Contains(u.GroupIDs, AdminGroupID)
}

// isSoleActiveAdminLocked reports whether id is the only active admin.
// Caller holds f.mu.
func (f *fsm) isSoleActiveAdminLocked(id string) bool {
	target, ok := f.users[id]
	if !ok || !isActiveAdmin(target) {
		return false
	}
	for otherID, other := range f.users {
		if otherID != id && isActiveAdmin(other) {
			return false
		}
	}
	return true
}

// isSoleActiveAdmin is isSoleActiveAdminLocked with a read lock, for
// pre-checks outside Apply.
func (f *fsm) isSoleActiveAdmin(id string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.isSoleActiveAdminLocked(id)
}

func (f *fsm) getUser(id string) (User, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	u, ok := f.users[id]
	return u, ok
}

// getUserByUsername is a linear scan; add an index if user counts grow.
func (f *fsm) getUserByUsername(username string) (User, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, u := range f.users {
		if u.Username == username {
			return u, true
		}
	}
	return User{}, false
}

func (f *fsm) listUsers() []User {
	f.mu.RLock()
	defer f.mu.RUnlock()
	users := make([]User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	return users
}

// UserRepository provides user operations; get one via Store.Users().
type UserRepository struct {
	store *Store
}

// Get returns a user, if it exists.
func (r UserRepository) Get(id string) (User, bool) {
	return r.store.fsm.getUser(id)
}

// GetByUsername looks up a user by their (unique) username.
func (r UserRepository) GetByUsername(username string) (User, bool) {
	return r.store.fsm.getUserByUsername(username)
}

// List returns all known users, ordered by ID.
func (r UserRepository) List() []User {
	return r.store.fsm.listUsers()
}

// Set creates or updates a user through Raft. Duplicate usernames and
// removing the last admin are pre-checked here; fsm.Apply enforces both.
func (r UserRepository) Set(user User) (User, error) {
	if existing, ok := r.store.fsm.getUserByUsername(user.Username); ok && existing.ID != user.ID {
		return User{}, ErrUsernameTaken
	}
	if r.store.fsm.isSoleActiveAdmin(user.ID) && !isActiveAdmin(user) {
		return User{}, ErrLastAdmin
	}

	resp, err := r.store.apply(command{Op: opSet, Entity: entityUser, User: &user})
	if err != nil {
		return User{}, err
	}
	switch v := resp.(type) {
	case User:
		return v, nil
	case error:
		return User{}, v
	default:
		return User{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// Delete removes a user by ID, except the last admin (ErrLastAdmin).
func (r UserRepository) Delete(id string) error {
	if r.store.fsm.isSoleActiveAdmin(id) {
		return ErrLastAdmin
	}

	resp, err := r.store.apply(command{Op: opDelete, Entity: entityUser, Key: id})
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}
