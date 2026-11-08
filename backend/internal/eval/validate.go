package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// ErrMalformed wraps every reason Validate rejects a definition.
var ErrMalformed = errors.New("malformed definition")

// schemaVersion is the only §6.1/§6.2 schemaVersion evaluator level 1 reads.
const schemaVersion = 1

// Value kinds (§6.3).
const (
	kindString = "string"
	kindNumber = "number"
	kindBool   = "bool"
)

// Validate reports why flag, together with the segments it references,
// is malformed (§7.1 step 2, §9.1), or nil. Evaluate returns
// malformed_flag for any non-nil result.
//
// shortcut: checks the §6 rules evaluation depends on, not the §2.2 count
// and id-pattern limits or JSON normal form; those are write-time
// validation (#146). Move them here if a caller ever evaluates
// definitions that skipped the HTTP API.
func Validate(flag Flag, segments map[string]Segment) error {
	if flag.SchemaVersion != schemaVersion {
		return malformed("schemaVersion %d", flag.SchemaVersion)
	}
	if segments == nil {
		segments = map[string]Segment{} // nil means "inside a segment" to validateGroup
	}
	if err := validateVariations(flag); err != nil {
		return err
	}
	variations := make(map[string]bool, len(flag.Variations))
	for _, v := range flag.Variations {
		variations[v.ID] = true
	}
	if !variations[flag.OffVariation] {
		return malformed("offVariation %q is not a variation", flag.OffVariation)
	}
	if err := validateServe(flag.Fallthrough, variations); err != nil {
		return fmt.Errorf("fallthrough: %w", err)
	}
	ruleIDs := make(map[string]bool, len(flag.Rules))
	for _, rule := range flag.Rules {
		if rule.ID == "" || ruleIDs[rule.ID] {
			return malformed("rule id %q is empty or duplicate", rule.ID)
		}
		ruleIDs[rule.ID] = true
		if err := validateGroup(rule.Match, rule.Conditions, segments); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
		if err := validateServe(rule.Serve, variations); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
	}
	return nil
}

func validateVariations(flag Flag) error {
	if len(flag.Variations) == 0 {
		return malformed("no variations")
	}
	if flag.Type == FlagTypeBoolean {
		if len(flag.Variations) != 2 ||
			flag.Variations[0].ID != "true" || !bytes.Equal(flag.Variations[0].Value, []byte("true")) ||
			flag.Variations[1].ID != "false" || !bytes.Equal(flag.Variations[1].Value, []byte("false")) {
			return malformed("boolean variations must be exactly true, false")
		}
		return nil
	}
	seen := make(map[string]bool, len(flag.Variations))
	for _, v := range flag.Variations {
		if v.ID == "" || seen[v.ID] {
			return malformed("variation id %q is empty or duplicate", v.ID)
		}
		seen[v.ID] = true
		if !valueFitsType(v.Value, flag.Type) {
			return malformed("variation %q value does not fit type %q", v.ID, flag.Type)
		}
	}
	return nil
}

func valueFitsType(raw json.RawMessage, t FlagType) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	switch t {
	case FlagTypeString:
		_, ok := value.(string)
		return ok
	case FlagTypeInteger:
		n, ok := value.(float64)
		return ok && n == math.Trunc(n) && math.Abs(n) <= maxSafeInteger
	case FlagTypeJSON:
		return true
	}
	return false
}

func validateServe(s Serve, variations map[string]bool) error {
	if (s.Variation == "") == (s.Rollout == nil) {
		return malformed("serve must be exactly one of variation or rollout")
	}
	if s.Rollout == nil {
		if !variations[s.Variation] {
			return malformed("served variation %q is not a variation", s.Variation)
		}
		return nil
	}
	seen := make(map[string]bool, len(s.Rollout.Weights))
	sum := 0
	for _, w := range s.Rollout.Weights {
		if !variations[w.Variation] || seen[w.Variation] {
			return malformed("rollout variation %q is unknown or duplicate", w.Variation)
		}
		seen[w.Variation] = true
		// Checked per weight so out-of-range values can't sum to 100000.
		if w.Weight < 0 || w.Weight > bucketCount {
			return malformed("rollout weight %d is outside 0–%d", w.Weight, bucketCount)
		}
		sum += w.Weight
	}
	if sum != bucketCount {
		return malformed("rollout weights sum to %d, not %d", sum, bucketCount)
	}
	return nil
}

// validateGroup checks a root group (§6.4) at evaluator level 1: depth 1,
// leaves only. segments is nil inside a segment, where inSegment is
// forbidden (§6.2); otherwise every referenced segment must be present and
// valid.
func validateGroup(match string, conditions []Condition, segments map[string]Segment) error {
	if match != "" && match != matchAll && match != matchAny {
		return malformed("match %q", match)
	}
	if len(conditions) == 0 {
		return malformed("empty group")
	}
	for _, c := range conditions {
		if err := validateLeaf(c, segments); err != nil {
			return err
		}
	}
	return nil
}

func validateLeaf(c Condition, segments map[string]Segment) error {
	if c.Match != "" || c.Conditions != nil {
		return malformed("nested group (evaluator level 1 allows depth 1)")
	}
	var wantKind string
	switch c.Op {
	case opIn:
	case opContains, opStartsWith, opEndsWith, opInSegment:
		wantKind = kindString
	case opLT, opLTE, opGT, opGTE:
		wantKind = kindNumber
	default:
		return malformed("unknown op %q", c.Op)
	}
	if (c.Op == opInSegment) != (c.Attribute == "") {
		return malformed("%s condition attribute %q", c.Op, c.Attribute)
	}
	if len(c.Values) == 0 {
		return malformed("%s condition has no values", c.Op)
	}
	kind := ""
	for _, raw := range c.Values {
		value, err := decodeValue(raw)
		if err != nil {
			return err
		}
		if kind == "" {
			kind = valueKind(value)
		}
		if valueKind(value) != kind || (wantKind != "" && kind != wantKind) {
			return malformed("%s condition values must all be one fitting value kind", c.Op)
		}
		if key, ok := value.(string); ok && c.Op == opInSegment {
			if err := validateSegmentRef(key, segments); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSegmentRef(key string, segments map[string]Segment) error {
	if segments == nil {
		return malformed("segment rules must not use inSegment")
	}
	segment, ok := segments[key]
	if !ok {
		return malformed("segment %q is not held", key)
	}
	if segment.SchemaVersion != schemaVersion {
		return malformed("segment %q schemaVersion %d", key, segment.SchemaVersion)
	}
	ruleIDs := make(map[string]bool, len(segment.Rules))
	for _, rule := range segment.Rules {
		if rule.ID == "" || ruleIDs[rule.ID] {
			return malformed("segment %q rule id %q is empty or duplicate", key, rule.ID)
		}
		ruleIDs[rule.ID] = true
		if err := validateGroup(rule.Match, rule.Conditions, nil); err != nil {
			return fmt.Errorf("segment %q rule %q: %w", key, rule.ID, err)
		}
	}
	return nil
}

// decodeValue decodes one condition value: a string, number or bool.
func decodeValue(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, malformed("condition value %s: %v", raw, err)
	}
	if valueKind(value) == "" {
		return nil, malformed("condition value %s is not a string, number or bool", raw)
	}
	return value, nil
}

func valueKind(value any) string {
	switch value.(type) {
	case string:
		return kindString
	case float64:
		return kindNumber
	case bool:
		return kindBool
	}
	return ""
}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformed, fmt.Sprintf(format, args...))
}
