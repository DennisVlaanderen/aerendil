package eval

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Operators (§6.3) and group matches (§6.4).
const (
	opIn         = "in"
	opContains   = "contains"
	opStartsWith = "startsWith"
	opEndsWith   = "endsWith"
	opLT         = "lt"
	opLTE        = "lte"
	opGT         = "gt"
	opGTE        = "gte"
	opInSegment  = "inSegment"

	matchAll = "all"
	matchAny = "any"
)

const (
	// targetingKeyAttribute names the context's targeting key in conditions
	// and bucketBy (§6.3, §6.5).
	targetingKeyAttribute = "targetingKey"
	// bucketCount is the number of buckets, and the sum of rollout weights.
	bucketCount = 100000
	// maxSafeInteger is 2^53−1, the largest integer every language's
	// float64 formats the same way (§6.5).
	maxSafeInteger = 1<<53 - 1
)

// Evaluate runs the §7.1 algorithm for flag (nil when the environment has
// no such flag) against ctx. segments holds the segments flag references,
// by key; a missing one makes the flag malformed. ctx is assumed valid
// (§6.6): the server rejects an invalid context before evaluating.
//
// shortcut: validates and decodes the definition on every call; cache a
// validated form per revision once evaluation is on a hot path.
func Evaluate(flag *Flag, segments map[string]Segment, ctx Context) Result {
	if flag == nil {
		return Result{Reason: ReasonFlagNotFound}
	}
	if Validate(*flag, segments) != nil {
		return Result{Reason: ReasonEvaluationError, ErrorKind: ErrorKindMalformedFlag}
	}
	if !flag.Enabled {
		return serveVariation(*flag, flag.OffVariation, Result{Reason: ReasonFlagDisabled})
	}
	for _, rule := range flag.Rules {
		if matchGroup(rule.Match, rule.Conditions, segments, ctx) {
			return serve(*flag, rule.Serve, ctx, Result{Reason: ReasonRuleMatch, RuleID: rule.ID})
		}
	}
	return serve(*flag, flag.Fallthrough, ctx, Result{Reason: ReasonFallthrough})
}

func serve(flag Flag, s Serve, ctx Context, result Result) Result {
	if s.Rollout == nil {
		return serveVariation(flag, s.Variation, result)
	}
	b := bucket(salt(flag), s.Rollout.BucketBy, ctx)
	cumulative := 0
	for _, w := range s.Rollout.Weights {
		cumulative += w.Weight
		if cumulative > b {
			return serveVariation(flag, w.Variation, result)
		}
	}
	panic("eval: rollout weights validated to sum to 100000")
}

func serveVariation(flag Flag, id string, result Result) Result {
	for _, v := range flag.Variations {
		if v.ID == id {
			result.VariationID, result.Value = v.ID, v.Value
			return result
		}
	}
	panic("eval: served variation validated to exist")
}

func salt(flag Flag) string {
	if flag.Salt == "" {
		return flag.Key
	}
	return flag.Salt
}

// bucket is the §6.5 bucket of ctx: 0 when the bucketBy attribute is
// missing or unusable.
func bucket(salt, bucketBy string, ctx Context) int {
	if bucketBy == "" {
		bucketBy = targetingKeyAttribute
	}
	value, ok := attribute(ctx, bucketBy)
	if !ok {
		return 0
	}
	var text string
	switch v := value.(type) {
	case string:
		text = v
	case float64:
		if v != math.Trunc(v) || math.Abs(v) > maxSafeInteger {
			return 0
		}
		text = strconv.FormatInt(int64(v), 10) // int64(-0) is 0, written "0"
	default:
		return 0
	}
	digest := sha256.Sum256([]byte(salt + "." + text))
	return int(binary.BigEndian.Uint64(digest[:8]) % bucketCount)
}

func attribute(ctx Context, name string) (any, bool) {
	if name == targetingKeyAttribute {
		return ctx.TargetingKey, true
	}
	value, ok := ctx.Attributes[name]
	return value, ok
}

// matchGroup evaluates a root group (§6.4); its children are leaves.
func matchGroup(match string, conditions []Condition, segments map[string]Segment, ctx Context) bool {
	if match == matchAny {
		return slices.ContainsFunc(conditions, func(c Condition) bool { return matchCondition(c, segments, ctx) })
	}
	for _, c := range conditions {
		if !matchCondition(c, segments, ctx) {
			return false
		}
	}
	return true
}

// matchCondition evaluates one leaf (§6.3). A missing attribute, or one
// whose value kind differs from the condition's, is false before negate.
func matchCondition(c Condition, segments map[string]Segment, ctx Context) bool {
	values := decodeValues(c.Values)
	if c.Op == opInSegment {
		member := slices.ContainsFunc(values, func(key any) bool {
			k, _ := key.(string) // Validate allows only string keys
			return isMember(segments[k], ctx)
		})
		return member != c.Negate
	}
	value, ok := attribute(ctx, c.Attribute)
	if !ok {
		return false
	}
	elements := []any{value}
	if list, isList := value.([]any); isList {
		elements = list // an empty list has no value kind: missing
	}
	fits, matched := false, false
	for _, element := range elements {
		if valueKind(element) != valueKind(values[0]) {
			continue
		}
		fits = true
		if slices.ContainsFunc(values, func(want any) bool { return compare(c.Op, element, want) }) {
			matched = true
			break
		}
	}
	return fits && matched != c.Negate
}

// compare applies op to one attribute element and one condition value of
// the same value kind.
func compare(op string, got, want any) bool {
	if op == opIn {
		return got == want // float64 ==, so -0 equals 0
	}
	if gotString, ok := got.(string); ok {
		wantString, _ := want.(string)
		switch op {
		case opContains:
			return strings.Contains(gotString, wantString)
		case opStartsWith:
			return strings.HasPrefix(gotString, wantString)
		case opEndsWith:
			return strings.HasSuffix(gotString, wantString)
		}
		return false
	}
	gotNumber, _ := got.(float64)
	wantNumber, _ := want.(float64)
	switch op {
	case opLT:
		return gotNumber < wantNumber
	case opLTE:
		return gotNumber <= wantNumber
	case opGT:
		return gotNumber > wantNumber
	case opGTE:
		return gotNumber >= wantNumber
	}
	return false
}

// isMember decides segment membership in §6.2 order.
func isMember(segment Segment, ctx Context) bool {
	if slices.Contains(segment.Included, ctx.TargetingKey) {
		return true
	}
	if slices.Contains(segment.Excluded, ctx.TargetingKey) {
		return false
	}
	return slices.ContainsFunc(segment.Rules, func(r SegmentRule) bool {
		return matchGroup(r.Match, r.Conditions, nil, ctx)
	})
}

// decodeValues decodes condition values Validate has already accepted.
func decodeValues(raws []json.RawMessage) []any {
	values := make([]any, len(raws))
	for i, raw := range raws {
		values[i], _ = decodeValue(raw)
	}
	return values
}
