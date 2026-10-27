package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// ErrSegmentInUse means a segment delete was refused because a flag in the
// same environment still references it via inSegment (fqdp.md §6.2).
var ErrSegmentInUse = errors.New("segment is referenced by a flag")

// ErrSegmentUsesInSegment means a segment rule used inSegment; segments must
// not recurse (fqdp.md §6.2).
var ErrSegmentUsesInSegment = errors.New("segment rules must not use inSegment")

// ErrSegmentLimitExceeded means a segment breaks a fqdp.md §2.2 limit.
var ErrSegmentLimitExceeded = errors.New("segment exceeds a limit")

// segmentSchemaVersion is the §6.2 schemaVersion the store writes.
const segmentSchemaVersion = 1

// Segment limits (fqdp.md §2.2).
const (
	maxSegmentRules         = 100
	maxSegmentTargetingKeys = 6000
)

// opInSegment is the condition op that references segments (§6.3).
const opInSegment = "inSegment"

// Segment is a reusable audience (fqdp.md §6.2) in one Environment; the
// same Key in another environment is a separate record (see flagMapKey).
// Condition contents are validated by the API, not here.
type Segment struct {
	SchemaVersion int    `json:"schemaVersion"`
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Name          string `json:"name"`
	// Revision is the Raft log index of the last write.
	Revision uint64 `json:"revision"`
	// EvaluatorLevel is the minimum evaluator level (§8.8), computed on write.
	EvaluatorLevel int           `json:"evaluatorLevel"`
	Included       []string      `json:"included"`
	Excluded       []string      `json:"excluded"`
	Rules          []SegmentRule `json:"rules"`
}

// SegmentRule is a root condition group (§6.4) with no serve.
type SegmentRule struct {
	ID         string      `json:"id"`
	Match      string      `json:"match,omitempty"`
	Conditions []Condition `json:"conditions"`
}

// segmentEvaluatorLevel is the §8.8 level a segment needs. Level 1 is all
// v0.1 defines; raise it here when nesting or new operators arrive.
func segmentEvaluatorLevel(Segment) int { return 1 }

func validateSegment(segment Segment) error {
	if len(segment.Rules) > maxSegmentRules {
		return fmt.Errorf("%w: %d rules, max %d", ErrSegmentLimitExceeded, len(segment.Rules), maxSegmentRules)
	}
	if n := len(segment.Included) + len(segment.Excluded); n > maxSegmentTargetingKeys {
		return fmt.Errorf("%w: %d included+excluded keys, max %d", ErrSegmentLimitExceeded, n, maxSegmentTargetingKeys)
	}
	for _, rule := range segment.Rules {
		if usesInSegment(rule.Conditions, nil) {
			return fmt.Errorf("%w: rule %q", ErrSegmentUsesInSegment, rule.ID)
		}
	}
	return nil
}

// usesInSegment reports whether any inSegment clause in conditions (groups
// included) names segmentKey, or any segment at all when segmentKey is nil.
func usesInSegment(conditions []Condition, segmentKey *string) bool {
	for _, condition := range conditions {
		if condition.Op == opInSegment {
			if segmentKey == nil {
				return true
			}
			for _, raw := range condition.Values {
				var key string
				if json.Unmarshal(raw, &key) == nil && key == *segmentKey {
					return true
				}
			}
		}
		if usesInSegment(condition.Conditions, segmentKey) {
			return true
		}
	}
	return false
}

// segmentReferencedLocked reports whether a flag in environmentID references
// segmentKey. Caller holds f.mu.
func (f *fsm) segmentReferencedLocked(environmentID, segmentKey string) bool {
	for _, flag := range f.flags {
		if flag.EnvironmentID != environmentID {
			continue
		}
		for _, rule := range flag.Rules {
			if usesInSegment(rule.Conditions, &segmentKey) {
				return true
			}
		}
	}
	return false
}

// hasSegmentsInEnvironmentLocked reports whether environmentID has segments.
// Caller holds f.mu.
func (f *fsm) hasSegmentsInEnvironmentLocked(environmentID string) bool {
	for _, segment := range f.segments {
		if segment.EnvironmentID == environmentID {
			return true
		}
	}
	return false
}

// hasSegmentsInEnvironment is hasSegmentsInEnvironmentLocked with a read lock.
func (f *fsm) hasSegmentsInEnvironment(environmentID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.hasSegmentsInEnvironmentLocked(environmentID)
}

func (f *fsm) applySegment(index uint64, cmd command) any {
	switch cmd.Op {
	case opSet:
		if _, ok := f.environments[cmd.Segment.EnvironmentID]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownEnvironment, cmd.Segment.EnvironmentID)
		}
		if err := validateSegment(*cmd.Segment); err != nil {
			return err
		}
		cmd.Segment.SchemaVersion = segmentSchemaVersion
		cmd.Segment.Revision = index
		cmd.Segment.EvaluatorLevel = segmentEvaluatorLevel(*cmd.Segment)
		f.segments[flagMapKey(cmd.Segment.EnvironmentID, cmd.Segment.Key)] = *cmd.Segment
		return *cmd.Segment
	case opDelete:
		segment, ok := f.segments[cmd.Key]
		if !ok {
			return nil
		}
		if f.segmentReferencedLocked(segment.EnvironmentID, segment.Key) {
			return fmt.Errorf("%w: %q in environment %q", ErrSegmentInUse, segment.Key, segment.EnvironmentID)
		}
		delete(f.segments, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

func (f *fsm) getSegment(environmentID, key string) (Segment, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	segment, ok := f.segments[flagMapKey(environmentID, key)]
	return segment, ok
}

func (f *fsm) listSegments(environmentID string) []Segment {
	f.mu.RLock()
	defer f.mu.RUnlock()
	segments := make([]Segment, 0)
	for _, segment := range f.segments {
		if segment.EnvironmentID == environmentID {
			segments = append(segments, segment)
		}
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i].Key < segments[j].Key })
	return segments
}

// SegmentRepository provides segment operations; get one via Store.Segments().
type SegmentRepository struct {
	store *Store
}

// Get returns a segment in an environment, if it exists.
func (r SegmentRepository) Get(environmentID, key string) (Segment, bool) {
	return r.store.fsm.getSegment(environmentID, key)
}

// List returns all segments scoped to environmentID, ordered by Key.
func (r SegmentRepository) List(environmentID string) []Segment {
	return r.store.fsm.listSegments(environmentID)
}

// Set applies a segment change through Raft; fails with
// ErrUnknownEnvironment, ErrSegmentLimitExceeded or ErrSegmentUsesInSegment.
func (r SegmentRepository) Set(segment Segment) (Segment, error) {
	resp, err := r.store.apply(command{Op: opSet, Entity: entitySegment, Segment: &segment})
	if err != nil {
		return Segment{}, err
	}
	switch v := resp.(type) {
	case Segment:
		return v, nil
	case error:
		return Segment{}, v
	default:
		return Segment{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// Delete removes a segment from an environment through Raft; fails with
// ErrSegmentInUse while a flag in that environment references it.
func (r SegmentRepository) Delete(environmentID, key string) error {
	resp, err := r.store.apply(command{Op: opDelete, Entity: entitySegment, Key: flagMapKey(environmentID, key)})
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}
