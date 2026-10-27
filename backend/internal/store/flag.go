package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// ErrUnknownEnvironment means a flag's EnvironmentID names no Environment.
var ErrUnknownEnvironment = errors.New("environment does not exist")

// ErrFlagExists means SetMany targeted a key that already exists in an
// environment; creating must never overwrite (that's Set, via PUT).
var ErrFlagExists = errors.New("flag already exists")

// ErrFlagTypeChanged means a Set tried to change an existing flag's Type,
// which fqdp.md §6.1 fixes at creation.
var ErrFlagTypeChanged = errors.New("flag type cannot be changed")

// ErrUnknownFlagType means a flag's Type is not one of the FlagType values.
var ErrUnknownFlagType = errors.New("unknown flag type")

// flagSchemaVersion is the §6.1 schemaVersion the store writes.
const flagSchemaVersion = 1

// FlagType is the value type every variation of a flag shares.
type FlagType string

// FlagType values (fqdp.md §6.1).
const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeInteger FlagType = "integer"
	FlagTypeString  FlagType = "string"
	FlagTypeJSON    FlagType = "json"
)

// Flag is a feature flag definition (fqdp.md §6.1) in one Environment; the
// same Key in another environment is a separate record (see flagMapKey).
// Rule contents are validated by the API, not here.
type Flag struct {
	SchemaVersion int      `json:"schemaVersion"`
	EnvironmentID string   `json:"environmentId"`
	Key           string   `json:"key"`
	Type          FlagType `json:"type"`
	Enabled       bool     `json:"enabled"`
	// Revision is the Raft log index of the last write.
	Revision uint64 `json:"revision"`
	// EvaluatorLevel is the minimum evaluator level (§8.8), computed on write.
	EvaluatorLevel int         `json:"evaluatorLevel"`
	Salt           string      `json:"salt"`
	Variations     []Variation `json:"variations"`
	OffVariation   string      `json:"offVariation"`
	Rules          []Rule      `json:"rules"`
	Fallthrough    Serve       `json:"fallthrough"`
}

// Variation is one value a flag can serve; Value's JSON kind must match the
// flag's Type.
type Variation struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

// Rule is a targeting rule: its own root condition group (§6.4) plus what
// to serve when it matches.
type Rule struct {
	ID          string      `json:"id"`
	Description string      `json:"description,omitempty"`
	Match       string      `json:"match,omitempty"`
	Conditions  []Condition `json:"conditions"`
	Serve       Serve       `json:"serve"`
}

// Condition is a clause (§6.3/§6.4): a leaf when Op is set, a group when
// Match is set. Groups are rejected at evaluator level 1 but kept in the
// shape so later levels need no schema change. Values stay raw JSON so
// integers past 2^53 survive the Raft decode (any would make them float64).
type Condition struct {
	Attribute  string            `json:"attribute,omitempty"`
	Op         string            `json:"op,omitempty"`
	Values     []json.RawMessage `json:"values,omitempty"`
	Negate     bool              `json:"negate,omitempty"`
	Match      string            `json:"match,omitempty"`
	Conditions []Condition       `json:"conditions,omitempty"`
}

// Serve is exactly one of a fixed Variation or a Rollout (§6.5).
type Serve struct {
	Variation string   `json:"variation,omitempty"`
	Rollout   *Rollout `json:"rollout,omitempty"`
}

// Rollout splits contexts across variations by bucket (§6.5).
type Rollout struct {
	BucketBy string          `json:"bucketBy,omitempty"`
	Weights  []RolloutWeight `json:"weights"`
}

// RolloutWeight is a variation's share of the 100000 buckets.
type RolloutWeight struct {
	Variation string `json:"variation"`
	Weight    int    `json:"weight"`
}

func isKnownFlagType(t FlagType) bool {
	switch t {
	case FlagTypeBoolean, FlagTypeInteger, FlagTypeString, FlagTypeJSON:
		return true
	}
	return false
}

// flagEvaluatorLevel is the §8.8 level a flag needs. Level 1 is all v0.1
// defines; raise it here when nesting or new operators arrive.
func flagEvaluatorLevel(Flag) int { return 1 }

// stampFlag fills the fields the store owns on every write.
func stampFlag(flag *Flag, index uint64) {
	flag.SchemaVersion = flagSchemaVersion
	flag.Revision = index
	flag.EvaluatorLevel = flagEvaluatorLevel(*flag)
	if flag.Salt == "" {
		flag.Salt = flag.Key
	}
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
		if !isKnownFlagType(cmd.Flag.Type) {
			return fmt.Errorf("%w: %q", ErrUnknownFlagType, cmd.Flag.Type)
		}
		// Enforced here, not only in the API, so no write path can retype a flag.
		if existing, ok := f.flags[flagMapKey(cmd.Flag.EnvironmentID, cmd.Flag.Key)]; ok && existing.Type != cmd.Flag.Type {
			return fmt.Errorf("%w: %q is %q", ErrFlagTypeChanged, cmd.Flag.Key, existing.Type)
		}
		stampFlag(cmd.Flag, index)
		f.flags[flagMapKey(cmd.Flag.EnvironmentID, cmd.Flag.Key)] = *cmd.Flag
		return *cmd.Flag
	case opSetBatch:
		// Validate the whole batch first so one bad entry rejects all of it.
		// The existence check lives here, not only in the API, so two
		// concurrent creates can't both pass a pre-check and overwrite.
		for _, flag := range cmd.Flags {
			if _, ok := f.environments[flag.EnvironmentID]; !ok {
				return fmt.Errorf("%w: %q", ErrUnknownEnvironment, flag.EnvironmentID)
			}
			if !isKnownFlagType(flag.Type) {
				return fmt.Errorf("%w: %q", ErrUnknownFlagType, flag.Type)
			}
			if _, ok := f.flags[flagMapKey(flag.EnvironmentID, flag.Key)]; ok {
				return fmt.Errorf("%w: %q in environment %q", ErrFlagExists, flag.Key, flag.EnvironmentID)
			}
		}
		applied := make([]Flag, len(cmd.Flags))
		for i, flag := range cmd.Flags {
			stampFlag(&flag, index)
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
// ErrUnknownEnvironment, and changing an existing flag's Type returns
// ErrFlagTypeChanged.
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

// SetMany creates definition in every listed environment atomically
// (ErrUnknownEnvironment if any ID is unknown, ErrFlagExists if the key
// already exists in any of them). One definition, not []Flag, so it can't
// diverge per environment; its EnvironmentID is ignored.
func (r FlagRepository) SetMany(definition Flag, environmentIDs []string) ([]Flag, error) {
	flags := make([]Flag, len(environmentIDs))
	for i, envID := range environmentIDs {
		flags[i] = definition
		flags[i].EnvironmentID = envID
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
