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

// Each Validate error also wraps one of these, so a caller (the HTTP API)
// can tell the user which rule the definition broke.
var (
	ErrSchemaVersion       = errors.New("unsupported schemaVersion")
	ErrFlagType            = errors.New("unknown flag type")
	ErrNoVariations        = errors.New("no variations")
	ErrVariationValue      = errors.New("variation value does not fit the flag type")
	ErrBooleanVariations   = errors.New("boolean variations must be exactly true, false")
	ErrDuplicateID         = errors.New("empty or duplicate id")
	ErrUnknownVariation    = errors.New("unknown variation")
	ErrServe               = errors.New("serve must be exactly one of variation or rollout")
	ErrRolloutWeights      = errors.New("invalid rollout weights")
	ErrGroup               = errors.New("invalid condition group")
	ErrNestedGroup         = errors.New("nested condition group")
	ErrCondition           = errors.New("invalid condition")
	ErrConditionValues     = errors.New("invalid condition values")
	ErrUnknownSegment      = errors.New("unknown segment")
	ErrSegmentUsesSegments = errors.New("segment rules must not use inSegment")
)

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
		return malformed(ErrSchemaVersion, "%d", flag.SchemaVersion)
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
		return malformed(ErrUnknownVariation, "offVariation %q", flag.OffVariation)
	}
	if err := validateServe(flag.Fallthrough, variations); err != nil {
		return fmt.Errorf("fallthrough: %w", err)
	}
	ruleIDs := make(map[string]bool, len(flag.Rules))
	for _, rule := range flag.Rules {
		if rule.ID == "" || ruleIDs[rule.ID] {
			return malformed(ErrDuplicateID, "rule id %q", rule.ID)
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
	switch flag.Type {
	case FlagTypeBoolean, FlagTypeInteger, FlagTypeString, FlagTypeJSON:
	default:
		return malformed(ErrFlagType, "%q", flag.Type)
	}
	if len(flag.Variations) == 0 {
		return malformed(ErrNoVariations, "")
	}
	if flag.Type == FlagTypeBoolean {
		if len(flag.Variations) != 2 ||
			flag.Variations[0].ID != "true" || !bytes.Equal(flag.Variations[0].Value, []byte("true")) ||
			flag.Variations[1].ID != "false" || !bytes.Equal(flag.Variations[1].Value, []byte("false")) {
			return malformed(ErrBooleanVariations, "")
		}
		return nil
	}
	seen := make(map[string]bool, len(flag.Variations))
	for _, v := range flag.Variations {
		if v.ID == "" || seen[v.ID] {
			return malformed(ErrDuplicateID, "variation id %q", v.ID)
		}
		seen[v.ID] = true
		if !valueFitsType(v.Value, flag.Type) {
			return malformed(ErrVariationValue, "variation %q, type %q", v.ID, flag.Type)
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
		return malformed(ErrServe, "")
	}
	if s.Rollout == nil {
		if !variations[s.Variation] {
			return malformed(ErrUnknownVariation, "served variation %q", s.Variation)
		}
		return nil
	}
	seen := make(map[string]bool, len(s.Rollout.Weights))
	sum := 0
	for _, w := range s.Rollout.Weights {
		if !variations[w.Variation] || seen[w.Variation] {
			return malformed(ErrUnknownVariation, "rollout variation %q is unknown or listed twice", w.Variation)
		}
		seen[w.Variation] = true
		// Checked per weight so out-of-range values can't sum to 100000.
		if w.Weight < 0 || w.Weight > bucketCount {
			return malformed(ErrRolloutWeights, "weight %d is outside 0–%d", w.Weight, bucketCount)
		}
		sum += w.Weight
	}
	if sum != bucketCount {
		return malformed(ErrRolloutWeights, "weights sum to %d, not %d", sum, bucketCount)
	}
	return nil
}

// validateGroup checks a root group (§6.4) at evaluator level 1: depth 1,
// leaves only. segments is nil inside a segment, where inSegment is
// forbidden (§6.2); otherwise every referenced segment must be present and
// valid.
func validateGroup(match string, conditions []Condition, segments map[string]Segment) error {
	if match != "" && match != matchAll && match != matchAny {
		return malformed(ErrGroup, "match %q", match)
	}
	if len(conditions) == 0 {
		return malformed(ErrGroup, "empty group")
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
		return malformed(ErrNestedGroup, "evaluator level 1 allows depth 1")
	}
	var wantKind string
	switch c.Op {
	case opIn:
	case opContains, opStartsWith, opEndsWith, opInSegment:
		wantKind = kindString
	case opLT, opLTE, opGT, opGTE:
		wantKind = kindNumber
	default:
		return malformed(ErrCondition, "unknown op %q", c.Op)
	}
	if (c.Op == opInSegment) != (c.Attribute == "") {
		return malformed(ErrCondition, "%s condition attribute %q", c.Op, c.Attribute)
	}
	if len(c.Values) == 0 {
		return malformed(ErrCondition, "%s condition has no values", c.Op)
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
			return malformed(ErrConditionValues, "%s condition values must all be one fitting value kind", c.Op)
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
		return malformed(ErrSegmentUsesSegments, "")
	}
	segment, ok := segments[key]
	if !ok {
		return malformed(ErrUnknownSegment, "%q", key)
	}
	if err := ValidateSegment(segment); err != nil {
		return fmt.Errorf("segment %q: %w", key, err)
	}
	return nil
}

// ValidateSegment reports why segment is malformed (§6.2), or nil. Validate
// runs it on every segment a flag references.
func ValidateSegment(segment Segment) error {
	if segment.SchemaVersion != schemaVersion {
		return malformed(ErrSchemaVersion, "%d", segment.SchemaVersion)
	}
	ruleIDs := make(map[string]bool, len(segment.Rules))
	for _, rule := range segment.Rules {
		if rule.ID == "" || ruleIDs[rule.ID] {
			return malformed(ErrDuplicateID, "rule id %q", rule.ID)
		}
		ruleIDs[rule.ID] = true
		if err := validateGroup(rule.Match, rule.Conditions, nil); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
	}
	return nil
}

// decodeValue decodes one condition value: a string, number or bool.
func decodeValue(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, malformed(ErrConditionValues, "%s: %v", raw, err)
	}
	if valueKind(value) == "" {
		return nil, malformed(ErrConditionValues, "%s is not a string, number or bool", raw)
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

func malformed(kind error, format string, args ...any) error {
	return fmt.Errorf("%w: %w: %s", ErrMalformed, kind, fmt.Sprintf(format, args...))
}
