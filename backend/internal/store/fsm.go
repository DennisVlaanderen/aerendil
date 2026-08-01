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
	entityFlag        entity = "flag"
	entityUser        entity = "user"
	entityGroup       entity = "group"
	entityAudit       entity = "audit"
	entityEnvironment entity = "environment"
)

// command is the single Raft log entry shape for every entity kind this
// store replicates; only the field matching Entity is populated. This is a
// breaking change to the previously flag-only, unversioned command/snapshot
// format -- acceptable pre-v1 with no real deployments to migrate, but it
// does mean an existing on-disk data dir must be wiped before running a
// build that includes this change.
type command struct {
	Op     		string 		`json:"op"`
	Entity 		entity 		`json:"entity"`
	Key    		string 		`json:"key,omitempty"`
	Flag   		*Flag  		`json:"flag,omitempty"`
	Flags       []Flag      `json:"flags,omitempty"`
	User   		*User  		`json:"user,omitempty"`
	Group  		*Group 		`json:"group,omitempty"`
	AuditEntry  *AuditEntry `json:"audit,omitempty"`
	Environment *Environment `json:"environment,omitempty"`
}

// fsm is the Raft finite state machine: in-memory maps of stored entitires, 
// made durable by Raft's own replicated log plus periodic snapshots.
// There's no need for a separate embedded database -- Raft already gives us a durable,
// replicated log to reconstruct all three from.
//
// Each entity's apply logic and read accessors live alongside that
// entity's struct definition (flag.go/user.go/group.go), not here -- this
// file only holds the machinery every entity shares: the command envelope,
// the Apply dispatch switch, and snapshot/restore. Adding a new entity
// means adding a file and minimal struct definition, not growing this one.
type fsm struct {
	mu     			sync.RWMutex
	flags  			map[string]Flag
	users  			map[string]User
	groups 			map[string]Group
	auditEntries 	map[uint64]AuditEntry
	environments 	map[string]Environment
}

func newFSM() *fsm {
	return &fsm{
		flags:  make(map[string]Flag),
		users:  make(map[string]User),
		groups: make(map[string]Group),
		auditEntries: make(map[uint64]AuditEntry),
		environments: make(map[string]Environment),
	}
}

// Apply is called once per committed log entry, in log order, on every
// node. Invariants that must hold cluster-wide (e.g. the Admin group's
// immutability) are enforced here rather than only in Store's pre-checks,
// since Apply is the one place guaranteed to run exactly once per entry.
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
	default:
		return fmt.Errorf("unknown command entity %q", cmd.Entity)
	}
}

// snapshotDoc is the single composite blob a whole FSM snapshot is encoded
// as -- one JSON document covering every entity, not per-entity files,
// following the same "snapshot the whole map" approach the original
// flag-only implementation used.
type snapshotDoc struct {
	Flags  map[string]Flag  `json:"flags"`
	Users  map[string]User  `json:"users"`
	Groups map[string]Group `json:"groups"`
	AuditEntries map[uint64]AuditEntry `json:"auditEntries"`
	Environments map[string]Environment `json:"environments"`
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	doc := snapshotDoc{
		Flags:  make(map[string]Flag, len(f.flags)),
		Users:  make(map[string]User, len(f.users)),
		Groups: make(map[string]Group, len(f.groups)),
		AuditEntries: make(map[uint64]AuditEntry, len(f.auditEntries)),
		Environments: make(map[string]Environment, len(f.environments)),
	}
	maps.Copy(doc.Flags, f.flags)
	maps.Copy(doc.Users, f.users)
	maps.Copy(doc.Groups, f.groups)
	maps.Copy(doc.AuditEntries, f.auditEntries)
	maps.Copy(doc.Environments, f.environments)
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
		// Detect a pre-user/group snapshot (a flat map[string]Flag, with no
		// "flags"/"users"/"groups" top-level keys at all) before decoding
		// into snapshotDoc -- otherwise it decodes "successfully" into three
		// nil maps, silently discarding every previously stored flag with no
		// error and no log line. A non-empty raw snapshot that matches
		// neither shape is exactly the breaking-change case the comment on
		// the command type above warns about; fail loudly instead of
		// quietly booting empty.
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
		// A pre-environment-scoped snapshot decodes "successfully" here
		// (Flag.EnvironmentID is just a new field, zero-valued to "" on
		// decode) but every flag would then be silently orphaned under
		// environment ID "" forever -- fail loudly instead, same "detect
		// the old shape, don't silently drop data" convention as the
		// pre-user/group probe above.
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
		// Not part of the pre-user/group legacy-format check above: an
		// existing snapshot legitimately has flags/users/groups but no
		// environments key yet, since Environment didn't exist when it was
		// written. That's a normal upgrade, not the unrecognized-format
		// case -- default to empty rather than failing loudly.
		doc.Environments = make(map[string]Environment)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.flags = doc.Flags
	f.users = doc.Users
	f.groups = doc.Groups
	f.auditEntries = doc.AuditEntries
	f.environments = doc.Environments
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
