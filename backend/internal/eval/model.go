// Package eval implements FQDP v0.1 flag evaluation (docs/fqdp.md §6.3–§6.5,
// §7). It is pure and has no dependency on the store, so the server's
// value-mode Query and dry-run evaluation and a future Go SDK can share it.
//
// The types mirror the §6 HTTP API / export JSON form, with the same member
// names as store.Flag and store.Segment, so a JSON round trip converts
// between them.
package eval

import "encoding/json"

// FlagType is the value type every variation of a flag shares (§6.1).
type FlagType string

// FlagType values (§6.1).
const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeInteger FlagType = "integer"
	FlagTypeString  FlagType = "string"
	FlagTypeJSON    FlagType = "json"
)

// Flag is the part of a flag definition (§6.1) that evaluation reads.
type Flag struct {
	SchemaVersion int         `json:"schemaVersion"`
	Key           string      `json:"key"`
	Type          FlagType    `json:"type"`
	Enabled       bool        `json:"enabled"`
	Salt          string      `json:"salt"`
	Variations    []Variation `json:"variations"`
	OffVariation  string      `json:"offVariation"`
	Rules         []Rule      `json:"rules"`
	Fallthrough   Serve       `json:"fallthrough"`
}

// Variation is one value a flag can serve.
type Variation struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

// Rule is a root condition group (§6.4) plus what to serve when it matches.
type Rule struct {
	ID         string      `json:"id"`
	Match      string      `json:"match,omitempty"`
	Conditions []Condition `json:"conditions"`
	Serve      Serve       `json:"serve"`
}

// Condition is a clause (§6.3/§6.4): a leaf when Op is set, a group when
// Match is set. Groups below the root are malformed at evaluator level 1.
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

// Segment is the part of a segment (§6.2) that membership reads.
type Segment struct {
	SchemaVersion int           `json:"schemaVersion"`
	Key           string        `json:"key"`
	Included      []string      `json:"included"`
	Excluded      []string      `json:"excluded"`
	Rules         []SegmentRule `json:"rules"`
}

// SegmentRule is a root condition group (§6.4) with no serve.
type SegmentRule struct {
	ID         string      `json:"id"`
	Match      string      `json:"match,omitempty"`
	Conditions []Condition `json:"conditions"`
}

// Context is what a flag is evaluated for (§6.6). Attribute values are as
// encoding/json decodes them into any: string, float64, bool, or []any of
// one of those. Any other Go type counts as a mismatched value kind.
type Context struct {
	TargetingKey string         `json:"targetingKey"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// Reason says why evaluation gave its result (§7.2).
type Reason string

// Reason values that travel on the wire (§7.2).
const (
	ReasonFlagDisabled    Reason = "flag_disabled"
	ReasonRuleMatch       Reason = "rule_match"
	ReasonFallthrough     Reason = "fallthrough"
	ReasonFlagNotFound    Reason = "flag_not_found"
	ReasonEvaluationError Reason = "evaluation_error"
)

// ErrorKind is the detail behind ReasonEvaluationError (§7.2).
type ErrorKind string

// ErrorKindMalformedFlag means the definition failed validation (§7.2). It
// is the only error kind that travels on the wire; "" is none.
const ErrorKindMalformedFlag ErrorKind = "malformed_flag"

// Result is the outcome of one evaluation (§7.2). When VariationID is
// empty, evaluation produced no value and the caller returns its fallback
// value.
type Result struct {
	Value       json.RawMessage `json:"value,omitempty"`
	VariationID string          `json:"variationId,omitempty"`
	Reason      Reason          `json:"reason"`
	RuleID      string          `json:"ruleId,omitempty"`
	ErrorKind   ErrorKind       `json:"errorKind,omitempty"`
}
