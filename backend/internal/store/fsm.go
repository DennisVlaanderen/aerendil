package store

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"sync"

	"github.com/hashicorp/raft"
)

const (
	opSet      = "set"
	opSetBatch = "setBatch"
	opDelete   = "delete"
	opAppend   = "append"
)

// entity discriminates which map a command applies to.
type entity string

const (
	entityFlag                  entity = "flag"
	entityUser                  entity = "user"
	entityGroup                 entity = "group"
	entityAudit                 entity = "audit"
	entityEnvironment           entity = "environment"
	entityApplicationCredential entity = "applicationCredential"
)

// command is the single Raft log entry shape for every entity; only the
// field matching Entity is populated. Schema changes here require wiping
// the Raft data dir on upgrade (see CLAUDE.md) -- no migration path pre-v1.
type command struct {
	Op                    string                 `json:"op"`
	Entity                entity                 `json:"entity"`
	Key                   string                 `json:"key,omitempty"`
	Flag                  *Flag                  `json:"flag,omitempty"`
	Flags                 []Flag                 `json:"flags,omitempty"`
	User                  *User                  `json:"user,omitempty"`
	Group                 *Group                 `json:"group,omitempty"`
	AuditEntry            *AuditEntry            `json:"audit,omitempty"`
	Environment           *Environment           `json:"environment,omitempty"`
	ApplicationCredential *ApplicationCredential `json:"applicationCredential,omitempty"`
}

// fsm is the Raft finite state machine: in-memory maps of stored entities,
// durable via Raft's replicated log plus periodic snapshots.
//
// Each entity's apply logic and accessors live alongside its struct
// definition (flag.go/user.go/group.go); this file only holds the shared
// machinery -- the command envelope, Apply dispatch, and snapshot/restore.
type fsm struct {
	mu                     sync.RWMutex
	flags                  map[string]Flag
	users                  map[string]User
	groups                 map[string]Group
	auditEntries           map[uint64]AuditEntry
	environments           map[string]Environment
	applicationCredentials map[string]ApplicationCredential
}

func newFSM() *fsm {
	return &fsm{
		flags:                  make(map[string]Flag),
		users:                  make(map[string]User),
		groups:                 make(map[string]Group),
		auditEntries:           make(map[uint64]AuditEntry),
		environments:           make(map[string]Environment),
		applicationCredentials: make(map[string]ApplicationCredential),
	}
}

// Apply is called once per committed log entry, in log order, on every
// node -- the one place guaranteed to run exactly once per entry, so
// cluster-wide invariants (e.g. Admin group immutability) are enforced here.
func (f *fsm) Apply(log *raft.Log) any {
	var cmd command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("decode command: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch cmd.Entity {
	case entityFlag:
		return f.applyFlag(log.Index, cmd)
	case entityUser:
		return f.applyUser(log.Index, cmd)
	case entityGroup:
		return f.applyGroup(log.Index, cmd)
	case entityAudit:
		return f.applyAuditEntry(log.Index, cmd)
	case entityEnvironment:
		return f.applyEnvironment(log.Index, cmd)
	case entityApplicationCredential:
		return f.applyApplicationCredential(log.Index, cmd)
	default:
		return fmt.Errorf("unknown command entity %q", cmd.Entity)
	}
}

// snapshotDoc is the single JSON document a whole FSM snapshot is encoded
// as -- one blob covering every entity, not per-entity files.
type snapshotDoc struct {
	Flags                  map[string]Flag                  `json:"flags"`
	Users                  map[string]User                  `json:"users"`
	Groups                 map[string]Group                 `json:"groups"`
	AuditEntries           map[uint64]AuditEntry            `json:"auditEntries"`
	Environments           map[string]Environment           `json:"environments"`
	ApplicationCredentials map[string]ApplicationCredential `json:"applicationCredentials"`
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	doc := snapshotDoc{
		Flags:                  make(map[string]Flag, len(f.flags)),
		Users:                  make(map[string]User, len(f.users)),
		Groups:                 make(map[string]Group, len(f.groups)),
		AuditEntries:           make(map[uint64]AuditEntry, len(f.auditEntries)),
		Environments:           make(map[string]Environment, len(f.environments)),
		ApplicationCredentials: make(map[string]ApplicationCredential, len(f.applicationCredentials)),
	}
	maps.Copy(doc.Flags, f.flags)
	maps.Copy(doc.Users, f.users)
	maps.Copy(doc.Groups, f.groups)
	maps.Copy(doc.AuditEntries, f.auditEntries)
	maps.Copy(doc.Environments, f.environments)
	maps.Copy(doc.ApplicationCredentials, f.applicationCredentials)
	return &fsmSnapshot{doc: doc}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	var doc snapshotDoc
	if len(raw) > 0 {
		// Detect a pre-user/group snapshot (flat map[string]Flag, no
		// top-level "flags"/"users"/"groups" keys) before decoding into
		// snapshotDoc -- otherwise it decodes "successfully" into nil maps,
		// silently dropping every stored flag. Fail loudly instead of
		// booting empty.
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			return fmt.Errorf("decode snapshot: %w", err)
		}
		_, hasFlags := probe["flags"]
		_, hasUsers := probe["users"]
		_, hasGroups := probe["groups"]
		if !hasFlags && !hasUsers && !hasGroups {
			return fmt.Errorf("snapshot is in an unrecognized (pre-user/group) format; wipe the raft data directory and restart so state rebuilds from a fresh snapshot")
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("decode snapshot: %w", err)
		}
	}
	for _, flag := range doc.Flags {
		// A pre-environment-scoped snapshot decodes "successfully"
		// (EnvironmentID zero-valued to "") but would silently orphan every
		// flag -- fail loudly instead, same convention as the pre-user/group
		// probe above.
		if flag.EnvironmentID == "" {
			return fmt.Errorf("snapshot has pre-environment-scoped flags; wipe the raft data directory and restart so state rebuilds from a fresh snapshot")
		}
	}
	if doc.Flags == nil {
		doc.Flags = make(map[string]Flag)
	}
	if doc.Users == nil {
		doc.Users = make(map[string]User)
	}
	if doc.Groups == nil {
		doc.Groups = make(map[string]Group)
	}
	if doc.AuditEntries == nil {
		doc.AuditEntries = make(map[uint64]AuditEntry)
	}
	if doc.Environments == nil {
		// Not the legacy-format case above: an older snapshot legitimately
		// has no environments key yet -- default to empty instead of failing.
		doc.Environments = make(map[string]Environment)
	}
	if doc.ApplicationCredentials == nil {
		// Same upgrade case as Environments above.
		doc.ApplicationCredentials = make(map[string]ApplicationCredential)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.flags = doc.Flags
	f.users = doc.Users
	f.groups = doc.Groups
	f.auditEntries = doc.AuditEntries
	f.environments = doc.Environments
	f.applicationCredentials = doc.ApplicationCredentials
	return nil
}

type fsmSnapshot struct {
	doc snapshotDoc
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if err := json.NewEncoder(sink).Encode(s.doc); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}
