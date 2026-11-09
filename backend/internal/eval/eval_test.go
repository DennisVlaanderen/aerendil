package eval

import (
	"encoding/json"
	"errors"
	"math"
	"os/exec"
	"strings"
	"testing"
)

func raws(t *testing.T, values ...any) []json.RawMessage {
	t.Helper()
	out := make([]json.RawMessage, len(values))
	for i, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = b
	}
	return out
}

// checkoutFlag is the fqdp.md §6.1 example.
func checkoutFlag(t *testing.T) Flag {
	return Flag{
		SchemaVersion: 1,
		Key:           "checkout.new-flow",
		Type:          FlagTypeString,
		Enabled:       true,
		Salt:          "checkout.new-flow",
		Variations: []Variation{
			{ID: "control", Value: json.RawMessage(`"v1"`)},
			{ID: "treatment", Value: json.RawMessage(`"v2"`)},
		},
		OffVariation: "control",
		Rules: []Rule{
			{
				ID:         "beta-testers",
				Match:      "all",
				Conditions: []Condition{{Op: "inSegment", Values: raws(t, "beta")}},
				Serve:      Serve{Variation: "treatment"},
			},
			{
				ID:         "benelux-50",
				Match:      "any",
				Conditions: []Condition{{Attribute: "country", Op: "in", Values: raws(t, "NL", "BE", "LU")}},
				Serve:      Serve{Rollout: fiftyFifty()},
			},
		},
		Fallthrough: Serve{Variation: "control"},
	}
}

func fiftyFifty() *Rollout {
	return &Rollout{BucketBy: "targetingKey", Weights: []RolloutWeight{
		{Variation: "control", Weight: 50000}, {Variation: "treatment", Weight: 50000},
	}}
}

// betaSegment is the fqdp.md §6.2 example.
func betaSegment(t *testing.T) Segment {
	return Segment{
		SchemaVersion: 1,
		Key:           "beta",
		Included:      []string{"user-1", "user-2"},
		Excluded:      []string{"user-9"},
		Rules: []SegmentRule{{
			ID:         "staff",
			Match:      "all",
			Conditions: []Condition{{Attribute: "email", Op: "endsWith", Values: raws(t, "@example.com")}},
		}},
	}
}

func segmentsOf(t *testing.T) map[string]Segment {
	return map[string]Segment{"beta": betaSegment(t)}
}

func TestEvaluateBucketingVectors(t *testing.T) {
	// §6.5 vectors, through the benelux-50 rollout.
	for key, want := range map[string]struct {
		bucket    int
		variation string
	}{
		"user-123": {74356, "treatment"},
		"user-456": {49880, "control"},
		"user-789": {21569, "control"},
	} {
		ctx := Context{TargetingKey: key, Attributes: map[string]any{"country": "NL"}}
		if got := bucket("checkout.new-flow", "targetingKey", ctx); got != want.bucket {
			t.Errorf("%s: bucket %d, want %d", key, got, want.bucket)
		}
		flag := checkoutFlag(t)
		got := Evaluate(&flag, segmentsOf(t), ctx)
		if got.VariationID != want.variation || got.Reason != ReasonRuleMatch || got.RuleID != "benelux-50" {
			t.Errorf("%s: got %+v, want %s via benelux-50", key, got, want.variation)
		}
	}
}

func TestBucketBy(t *testing.T) {
	salt := "checkout.new-flow"
	zero := bucket(salt, "id", Context{TargetingKey: "k", Attributes: map[string]any{"id": "0"}})
	for name, value := range map[string]any{
		"negative zero is written 0": math.Copysign(0, -1),
		"plain zero":                 0.0,
	} {
		if got := bucket(salt, "id", Context{TargetingKey: "k", Attributes: map[string]any{"id": value}}); got != zero {
			t.Errorf("%s: bucket %d, want %d (same as \"0\")", name, got, zero)
		}
	}
	if got, want := bucket(salt, "n", Context{TargetingKey: "k", Attributes: map[string]any{"n": 42.0}}),
		bucket(salt, "n", Context{TargetingKey: "k", Attributes: map[string]any{"n": "42"}}); got != want {
		t.Errorf("integer number 42: bucket %d, want %d (same as \"42\")", got, want)
	}
	for name, value := range map[string]any{
		"2^60":       math.Pow(2, 60),
		"fraction":   1.5,
		"boolean":    true,
		"list":       []any{"user-123"},
		"other type": 42, // an int, not a decoded float64
	} {
		if got := bucket(salt, "id", Context{TargetingKey: "k", Attributes: map[string]any{"id": value}}); got != 0 {
			t.Errorf("%s: bucket %d, want 0 (unusable)", name, got)
		}
	}
	if got := bucket(salt, "missing", Context{TargetingKey: "k"}); got != 0 {
		t.Errorf("missing bucketBy: bucket %d, want 0", got)
	}
	if got, want := bucket(salt, "", Context{TargetingKey: "user-123"}), 74356; got != want {
		t.Errorf("empty bucketBy defaults to targetingKey: bucket %d, want %d", got, want)
	}
}

func TestRolloutBoundaries(t *testing.T) {
	flag := checkoutFlag(t)
	flag.Rules = nil
	ctx := Context{TargetingKey: "user-456"} // bucket 49880

	flag.Fallthrough = Serve{Rollout: &Rollout{Weights: []RolloutWeight{
		{Variation: "control", Weight: 49880}, {Variation: "treatment", Weight: 50120},
	}}}
	if got := Evaluate(&flag, nil, ctx); got.VariationID != "treatment" {
		t.Errorf("bucket on a cumulative boundary: got %q, want the next variation", got.VariationID)
	}

	flag.Fallthrough = Serve{Rollout: &Rollout{BucketBy: "beta", Weights: []RolloutWeight{
		{Variation: "control", Weight: 0}, {Variation: "treatment", Weight: 100000},
	}}}
	ctx.Attributes = map[string]any{"beta": true} // unusable: bucket 0
	if got := Evaluate(&flag, nil, ctx); got.VariationID != "treatment" {
		t.Errorf("zero-weight variation at bucket 0: got %q, want it never served", got.VariationID)
	}
}

func TestConditions(t *testing.T) {
	ctx := Context{TargetingKey: "user-1", Attributes: map[string]any{
		"country": "NL",
		"email":   "ana@example.com",
		"age":     30.0,
		"zero":    0.0,
		"beta":    true,
		"groups":  []any{"a", "b"},
		"scores":  []any{1.0, 5.0},
		"empty":   []any{},
		"code":    "5",
	}}
	tests := []struct {
		name string
		cond Condition
		want bool
	}{
		{"in string", Condition{Attribute: "country", Op: "in", Values: raws(t, "BE", "NL")}, true},
		{"in string no match", Condition{Attribute: "country", Op: "in", Values: raws(t, "BE")}, false},
		{"in is case-sensitive", Condition{Attribute: "country", Op: "in", Values: raws(t, "nl")}, false},
		{"in number", Condition{Attribute: "age", Op: "in", Values: raws(t, 30)}, true},
		{"in -0 equals 0", Condition{Attribute: "zero", Op: "in", Values: []json.RawMessage{json.RawMessage(`-0`)}}, true},
		{"in bool", Condition{Attribute: "beta", Op: "in", Values: raws(t, true)}, true},
		{"in targetingKey", Condition{Attribute: "targetingKey", Op: "in", Values: raws(t, "user-1")}, true},
		{"in is strict: \"5\" is not 5", Condition{Attribute: "code", Op: "in", Values: raws(t, 5)}, false},
		{"contains", Condition{Attribute: "email", Op: "contains", Values: raws(t, "zz", "@exam")}, true},
		{"contains no match", Condition{Attribute: "email", Op: "contains", Values: raws(t, "zz")}, false},
		{"startsWith", Condition{Attribute: "email", Op: "startsWith", Values: raws(t, "ana@")}, true},
		{"startsWith no match", Condition{Attribute: "email", Op: "startsWith", Values: raws(t, "bob@")}, false},
		{"endsWith", Condition{Attribute: "email", Op: "endsWith", Values: raws(t, "@example.com")}, true},
		{"endsWith no match", Condition{Attribute: "email", Op: "endsWith", Values: raws(t, "@example.org")}, false},
		{"lt", Condition{Attribute: "age", Op: "lt", Values: raws(t, 31)}, true},
		{"lt equal", Condition{Attribute: "age", Op: "lt", Values: raws(t, 30)}, false},
		{"lte equal", Condition{Attribute: "age", Op: "lte", Values: raws(t, 30)}, true},
		{"lte below", Condition{Attribute: "age", Op: "lte", Values: raws(t, 29)}, false},
		{"gt", Condition{Attribute: "age", Op: "gt", Values: raws(t, 29)}, true},
		{"gt equal", Condition{Attribute: "age", Op: "gt", Values: raws(t, 30)}, false},
		{"gte equal", Condition{Attribute: "age", Op: "gte", Values: raws(t, 30)}, true},
		{"gte above", Condition{Attribute: "age", Op: "gte", Values: raws(t, 31)}, false},
		{"gt any of values", Condition{Attribute: "age", Op: "gt", Values: raws(t, 50, 10)}, true},
		{"inSegment member", Condition{Op: "inSegment", Values: raws(t, "beta")}, true},

		{"negate present", Condition{Attribute: "country", Op: "in", Values: raws(t, "BE"), Negate: true}, true},
		{"negate present match", Condition{Attribute: "country", Op: "in", Values: raws(t, "NL"), Negate: true}, false},
		{"missing attribute", Condition{Attribute: "plan", Op: "in", Values: raws(t, "pro")}, false},
		{"negate missing attribute stays false", Condition{Attribute: "plan", Op: "in", Values: raws(t, "pro"), Negate: true}, false},
		{"mismatched kind is false", Condition{Attribute: "country", Op: "lt", Values: raws(t, 5)}, false},
		{"negate mismatched kind stays false", Condition{Attribute: "country", Op: "lt", Values: raws(t, 5), Negate: true}, false},
		{"negate mismatched kind for in stays false", Condition{Attribute: "code", Op: "in", Values: raws(t, 5), Negate: true}, false},
		{"negate inSegment member", Condition{Op: "inSegment", Values: raws(t, "beta"), Negate: true}, false},

		{"list any element", Condition{Attribute: "groups", Op: "in", Values: raws(t, "b")}, true},
		{"list no element", Condition{Attribute: "groups", Op: "in", Values: raws(t, "c")}, false},
		{"list negate inverts the overall result", Condition{Attribute: "groups", Op: "in", Values: raws(t, "a"), Negate: true}, false},
		{"list negate no element", Condition{Attribute: "groups", Op: "in", Values: raws(t, "c"), Negate: true}, true},
		{"number list", Condition{Attribute: "scores", Op: "gte", Values: raws(t, 5)}, true},
		{"empty list is missing", Condition{Attribute: "empty", Op: "in", Values: raws(t, "a")}, false},
		{"empty list with negate stays false", Condition{Attribute: "empty", Op: "in", Values: raws(t, "a"), Negate: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateLeaf(tt.cond, segmentsOf(t)); err != nil {
				t.Fatalf("condition should be valid: %v", err)
			}
			if got := matchCondition(tt.cond, segmentsOf(t), ctx); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNegateInSegmentMatchesNonMembers(t *testing.T) {
	cond := Condition{Op: "inSegment", Values: raws(t, "beta"), Negate: true}
	if !matchCondition(cond, segmentsOf(t), Context{TargetingKey: "stranger"}) {
		t.Error("negated inSegment should match a non-member")
	}
}

func TestSegmentMembershipOrder(t *testing.T) {
	segment := betaSegment(t)
	segment.Included = append(segment.Included, "both")
	segment.Excluded = append(segment.Excluded, "both")
	staff := map[string]any{"email": "x@example.com"}
	tests := []struct {
		name string
		ctx  Context
		want bool
	}{
		{"included", Context{TargetingKey: "user-1"}, true},
		{"included and excluded: included wins", Context{TargetingKey: "both"}, true},
		{"excluded beats a matching rule", Context{TargetingKey: "user-9", Attributes: staff}, false},
		{"matching rule", Context{TargetingKey: "user-5", Attributes: staff}, true},
		{"nothing matches", Context{TargetingKey: "user-5"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMember(segment, tt.ctx); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroups(t *testing.T) {
	nl := Condition{Attribute: "country", Op: "in", Values: raws(t, "NL")}
	pro := Condition{Attribute: "plan", Op: "in", Values: raws(t, "pro")}
	ctx := Context{TargetingKey: "k", Attributes: map[string]any{"country": "NL", "plan": "free"}}
	for _, tt := range []struct {
		match string
		want  bool
	}{{"all", false}, {"", false}, {"any", true}} {
		if got := matchGroup(tt.match, []Condition{nl, pro}, nil, ctx); got != tt.want {
			t.Errorf("match %q: got %v, want %v", tt.match, got, tt.want)
		}
	}
}

func TestEvaluateSteps(t *testing.T) {
	member := Context{TargetingKey: "user-1", Attributes: map[string]any{"country": "NL"}}
	tests := []struct {
		name   string
		flag   func(*Flag)
		ctx    Context
		want   Result
		absent bool
	}{
		{name: "flag missing", absent: true, ctx: member, want: Result{Reason: ReasonFlagNotFound}},
		{
			name: "kill switch skips rules", flag: func(f *Flag) { f.Enabled = false }, ctx: member,
			want: Result{Value: json.RawMessage(`"v1"`), VariationID: "control", Reason: ReasonFlagDisabled},
		},
		{
			name: "first matching rule wins", ctx: member,
			want: Result{Value: json.RawMessage(`"v2"`), VariationID: "treatment", Reason: ReasonRuleMatch, RuleID: "beta-testers"},
		},
		{
			name: "fallthrough", ctx: Context{TargetingKey: "user-5", Attributes: map[string]any{"country": "DE"}},
			want: Result{Value: json.RawMessage(`"v1"`), VariationID: "control", Reason: ReasonFallthrough},
		},
		{
			name: "malformed", flag: func(f *Flag) { f.OffVariation = "nope" }, ctx: member,
			want: Result{Reason: ReasonEvaluationError, ErrorKind: ErrorKindMalformedFlag},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flag *Flag
			if !tt.absent {
				f := checkoutFlag(t)
				flag = &f
				if tt.flag != nil {
					tt.flag(flag)
				}
			}
			got := Evaluate(flag, segmentsOf(t), tt.ctx)
			if string(got.Value) != string(tt.want.Value) || got.VariationID != tt.want.VariationID ||
				got.Reason != tt.want.Reason || got.RuleID != tt.want.RuleID || got.ErrorKind != tt.want.ErrorKind {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateRejects(t *testing.T) {
	leaf := func(f *Flag, c Condition) { f.Rules[1].Conditions = []Condition{c} }
	tests := []struct {
		name     string
		kind     error
		flag     func(*Flag)
		segments func(map[string]Segment)
	}{
		{name: "unknown schemaVersion", kind: ErrSchemaVersion, flag: func(f *Flag) { f.SchemaVersion = 2 }},
		{name: "unknown type", kind: ErrFlagType, flag: func(f *Flag) { f.Type = "float" }},
		{name: "no variations", kind: ErrNoVariations, flag: func(f *Flag) { f.Variations = nil }},
		{name: "duplicate variation id", kind: ErrDuplicateID, flag: func(f *Flag) { f.Variations[1].ID = "control" }},
		{name: "value does not fit type", kind: ErrVariationValue, flag: func(f *Flag) { f.Variations[0].Value = json.RawMessage(`1`) }},
		{name: "integer with a fraction", kind: ErrVariationValue, flag: func(f *Flag) {
			f.Type = FlagTypeInteger
			f.Variations[0].Value, f.Variations[1].Value = json.RawMessage(`1`), json.RawMessage(`1.5`)
		}},
		{name: "boolean table not true,false", kind: ErrBooleanVariations, flag: func(f *Flag) {
			f.Type = FlagTypeBoolean
			f.Variations = []Variation{{ID: "false", Value: json.RawMessage(`false`)}, {ID: "true", Value: json.RawMessage(`true`)}}
			f.OffVariation = "false"
		}},
		{name: "unknown offVariation", kind: ErrUnknownVariation, flag: func(f *Flag) { f.OffVariation = "nope" }},
		{name: "serve with both", kind: ErrServe, flag: func(f *Flag) { f.Fallthrough.Rollout = fiftyFifty() }},
		{name: "serve with neither", kind: ErrServe, flag: func(f *Flag) { f.Fallthrough = Serve{} }},
		{name: "weights not summing to 100000", kind: ErrRolloutWeights, flag: func(f *Flag) { f.Rules[1].Serve.Rollout.Weights[0].Weight = 40000 }},
		{name: "wrapping weights", kind: ErrRolloutWeights, flag: func(f *Flag) {
			f.Rules[1].Serve.Rollout.Weights[0].Weight = 4294967295
			f.Rules[1].Serve.Rollout.Weights[1].Weight = 100001
		}},
		{name: "negative weight", kind: ErrRolloutWeights, flag: func(f *Flag) {
			f.Rules[1].Serve.Rollout.Weights[0].Weight = -1
			f.Rules[1].Serve.Rollout.Weights[1].Weight = 100001
		}},
		{name: "duplicate rollout variation", kind: ErrUnknownVariation, flag: func(f *Flag) { f.Rules[1].Serve.Rollout.Weights[1].Variation = "control" }},
		{name: "duplicate rule id", kind: ErrDuplicateID, flag: func(f *Flag) { f.Rules[1].ID = "beta-testers" }},
		{name: "unknown match", kind: ErrGroup, flag: func(f *Flag) { f.Rules[1].Match = "none" }},
		{name: "empty group", kind: ErrGroup, flag: func(f *Flag) { f.Rules[1].Conditions = nil }},
		{name: "nested group", kind: ErrNestedGroup, flag: func(f *Flag) {
			leaf(f, Condition{Match: "all", Conditions: f.Rules[1].Conditions})
		}},
		{name: "leaf and group at once", kind: ErrNestedGroup, flag: func(f *Flag) { f.Rules[1].Conditions[0].Match = "all" }},
		{name: "unknown op", kind: ErrCondition, flag: func(f *Flag) { leaf(f, Condition{Attribute: "a", Op: "matches", Values: raws(t, "x")}) }},
		{name: "no values", kind: ErrCondition, flag: func(f *Flag) { leaf(f, Condition{Attribute: "a", Op: "in"}) }},
		{name: "mixed value kinds", kind: ErrConditionValues, flag: func(f *Flag) { leaf(f, Condition{Attribute: "a", Op: "in", Values: raws(t, "x", 1)}) }},
		{name: "lt with strings", kind: ErrConditionValues, flag: func(f *Flag) { leaf(f, Condition{Attribute: "a", Op: "lt", Values: raws(t, "x")}) }},
		{name: "contains with numbers", kind: ErrConditionValues, flag: func(f *Flag) { leaf(f, Condition{Attribute: "a", Op: "contains", Values: raws(t, 1)}) }},
		{name: "null value", kind: ErrConditionValues, flag: func(f *Flag) { leaf(f, Condition{Attribute: "a", Op: "in", Values: raws(t, nil)}) }},
		{name: "leaf without attribute", kind: ErrCondition, flag: func(f *Flag) { leaf(f, Condition{Op: "in", Values: raws(t, "x")}) }},
		{name: "inSegment with attribute", kind: ErrCondition, flag: func(f *Flag) { f.Rules[0].Conditions[0].Attribute = "a" }},
		{name: "segment not held", kind: ErrUnknownSegment, segments: func(s map[string]Segment) { delete(s, "beta") }},
		{name: "segment uses inSegment", kind: ErrSegmentUsesSegments, segments: func(s map[string]Segment) {
			seg := s["beta"]
			seg.Rules[0].Conditions = []Condition{{Op: "inSegment", Values: raws(t, "beta")}}
			s["beta"] = seg
		}},
		{name: "invalid segment rule", kind: ErrGroup, segments: func(s map[string]Segment) {
			seg := s["beta"]
			seg.Rules[0].Conditions = nil
			s["beta"] = seg
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag, segments := checkoutFlag(t), segmentsOf(t)
			if tt.flag != nil {
				tt.flag(&flag)
			}
			if tt.segments != nil {
				tt.segments(segments)
			}
			if err := Validate(flag, segments); !errors.Is(err, ErrMalformed) || !errors.Is(err, tt.kind) {
				t.Fatalf("expected ErrMalformed and %v, got %v", tt.kind, err)
			}
			if got := Evaluate(&flag, segments, Context{TargetingKey: "user-1"}); got.Reason != ReasonEvaluationError ||
				got.ErrorKind != ErrorKindMalformedFlag || got.VariationID != "" {
				t.Errorf("expected malformed_flag with no value, got %+v", got)
			}
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	if err := Validate(checkoutFlag(t), segmentsOf(t)); err != nil {
		t.Fatalf("§6.1 example should be valid: %v", err)
	}
	flag := checkoutFlag(t)
	flag.Rules = flag.Rules[1:]
	if err := Validate(flag, nil); err != nil {
		t.Fatalf("a flag without inSegment needs no segments: %v", err)
	}
	flag.Type = FlagTypeBoolean
	flag.Variations = []Variation{{ID: "true", Value: json.RawMessage(`true`)}, {ID: "false", Value: json.RawMessage(`false`)}}
	flag.OffVariation, flag.Fallthrough = "false", Serve{Variation: "true"}
	flag.Rules[0].Serve.Rollout.Weights = []RolloutWeight{{Variation: "true", Weight: 100000}}
	if err := Validate(flag, nil); err != nil {
		t.Fatalf("boolean flag should be valid: %v", err)
	}
}

func TestDoesNotImportStore(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	if strings.Contains(string(out), "backend/internal/store") {
		t.Fatal("eval must not depend on backend/internal/store")
	}
}
