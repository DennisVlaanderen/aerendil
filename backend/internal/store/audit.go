package store

import (
	"fmt"
	"sort"
	"time"
)

// AuditEntry is a durable record of a mutating API request, successful or
// rejected. ID is the Raft log index. Timestamp is set once by
// AuditRepository.Append, not applyAuditEntry, since Raft replays that
// entry on every node and time.Now() there would diverge across replicas.
type AuditEntry struct {
	ID         uint64 `json:"id"`
	Timestamp  int64  `json:"timestamp"`
	ActorID    string `json:"actorId"`
	ActorType  string `json:"actorType,omitempty"`
	Action     string `json:"action"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId,omitempty"`
	Before     string `json:"before,omitempty"`
	After      string `json:"after,omitempty"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error,omitempty"`
}

func (f *fsm) applyAuditEntry(index uint64, cmd command) interface{} {
	switch cmd.Op {
	case opAppend:
		cmd.AuditEntry.ID = index
		f.auditEntries[cmd.AuditEntry.ID] = *cmd.AuditEntry
		return *cmd.AuditEntry
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

// AuditFilter narrows AuditRepository.List. Empty-string fields are
// wildcards; non-empty fields are AND-combined.
type AuditFilter struct {
	TargetType string
	TargetID   string
	ActorID    string
}

func (af AuditFilter) matches(e AuditEntry) bool {
	if af.TargetType != "" && e.TargetType != af.TargetType {
		return false
	}
	if af.TargetID != "" && e.TargetID != af.TargetID {
		return false
	}
	if af.ActorID != "" && e.ActorID != af.ActorID {
		return false
	}
	return true
}

func (f *fsm) getAuditEntry(id uint64) (AuditEntry, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.auditEntries[id]
	return e, ok
}

// listAuditEntries returns entries matching filter, newest first -- ID is
// the Raft log index, so it doubles as a chronological key.
func (f *fsm) listAuditEntries(filter AuditFilter) []AuditEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	matched := make([]AuditEntry, 0, len(f.auditEntries))
	for _, e := range f.auditEntries {
		if filter.matches(e) {
			matched = append(matched, e)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })
	return matched
}

// AuditRepository provides audit-log operations against the store. Obtain
// one via Store.Audits().
type AuditRepository struct {
	store *Store
}

// Get returns a single audit entry by ID (== the Raft log index it
// committed at), if it exists.
func (r AuditRepository) Get(id uint64) (AuditEntry, bool) {
	return r.store.fsm.getAuditEntry(id)
}

// List returns audit entries matching filter, newest first.
func (r AuditRepository) List(filter AuditFilter) []AuditEntry {
	return r.store.fsm.listAuditEntries(filter)
}

// Append durably records entry through Raft consensus. Timestamp is
// assigned here, once, on the leader -- see the AuditEntry doc comment.
func (r AuditRepository) Append(entry AuditEntry) (AuditEntry, error) {
	entry.Timestamp = time.Now().Unix()
	resp, err := r.store.apply(command{Op: opAppend, Entity: entityAudit, AuditEntry: &entry})
	if err != nil {
		return AuditEntry{}, err
	}
	applied, ok := resp.(AuditEntry)
	if !ok {
		return AuditEntry{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
	return applied, nil
}
