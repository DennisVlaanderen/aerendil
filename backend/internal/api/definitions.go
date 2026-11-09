package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	"aerendil/backend/internal/eval"
	"aerendil/backend/internal/store"
)

// Patterns and limits from docs/fqdp.md §2.2 that eval.Validate leaves to
// write time. Segment keys reuse the flag-key pattern.
var (
	definitionKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	definitionIDPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	attributeNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
)

const (
	maxVariations           = 100
	maxFlagRules            = 500
	maxGroupChildren        = 50
	maxConditionValues      = 1000
	maxTargetingKeyBytes    = 256
	maxContextAttributes    = 100
	targetingKeyAttribute   = "targetingKey"
	definitionSchemaVersion = 1
)

// evalErrorCodes maps each eval.Validate reason to the code the user sees.
// eval.ErrSchemaVersion is absent on purpose: the API always writes version
// 1, so it would be a server bug and falls through to a 500.
var evalErrorCodes = []struct {
	err  error
	code string
}{
	{eval.ErrFlagType, CodeBadRequestFlagTypeInvalid},
	{eval.ErrNoVariations, CodeBadRequestFlagVariationsRequired},
	{eval.ErrVariationValue, CodeBadRequestFlagVariationValueInvalid},
	{eval.ErrBooleanVariations, CodeBadRequestFlagBooleanVariations},
	{eval.ErrDuplicateID, CodeBadRequestDefinitionIDDuplicate},
	{eval.ErrUnknownVariation, CodeBadRequestFlagVariationUnknown},
	{eval.ErrServe, CodeBadRequestFlagServeInvalid},
	{eval.ErrRolloutWeights, CodeBadRequestFlagRolloutWeightsInvalid},
	{eval.ErrGroup, CodeBadRequestConditionGroupInvalid},
	{eval.ErrNestedGroup, CodeBadRequestConditionGroupNested},
	{eval.ErrCondition, CodeBadRequestConditionInvalid},
	{eval.ErrConditionValues, CodeBadRequestConditionValuesInvalid},
	{eval.ErrUnknownSegment, CodeBadRequestSegmentUnknown},
	{eval.ErrSegmentUsesSegments, CodeBadRequestSegmentUsesInSegment},
}

func evalErrorToAPIError(err error) error {
	for _, m := range evalErrorCodes {
		if errors.Is(err, m.err) {
			return badRequest(m.code, err.Error())
		}
	}
	return err
}

// toEval converts a store definition to its eval form; the two share their
// JSON member names, so a round trip drops only the store-only fields.
func toEval[T any](v any) (T, error) {
	var out T
	b, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(b, &out)
	return out, err
}

// evalSegments returns environmentID's segments in eval form, by key.
func evalSegments(environmentID string) (map[string]eval.Segment, error) {
	segments := map[string]eval.Segment{}
	for _, s := range dataStore.Segments().List(environmentID) {
		converted, err := toEval[eval.Segment](s)
		if err != nil {
			return nil, err
		}
		segments[s.Key] = converted
	}
	return segments, nil
}

// validateFlag checks flag, as it would be written to its EnvironmentID,
// against §2.2 and §6. It sets SchemaVersion, which the store also stamps.
func validateFlag(flag *store.Flag) error {
	flag.SchemaVersion = definitionSchemaVersion
	if !definitionKeyPattern.MatchString(flag.Key) {
		return badRequest(CodeBadRequestFlagKeyInvalid, fmt.Sprintf("key %q must match %s", flag.Key, definitionKeyPattern))
	}
	if len(flag.Variations) > maxVariations || len(flag.Rules) > maxFlagRules {
		return badRequest(CodeBadRequestDefinitionLimitExceeded, fmt.Sprintf("at most %d variations and %d rules", maxVariations, maxFlagRules))
	}
	for _, v := range flag.Variations {
		if err := checkID("variation", v.ID); err != nil {
			return err
		}
	}
	for _, rule := range flag.Rules {
		if err := checkRule(rule.ID, rule.Conditions); err != nil {
			return err
		}
	}

	evalFlag, err := toEval[eval.Flag](flag)
	if err != nil {
		return err
	}
	segments, err := evalSegments(flag.EnvironmentID)
	if err != nil {
		return err
	}
	return evalErrorToAPIError(eval.Validate(evalFlag, segments))
}

// validateSegment checks segment against §2.2 and §6.2. The store enforces
// the rule and targeting-key count limits. It sets SchemaVersion, which the
// store also stamps.
func validateSegment(segment *store.Segment) error {
	segment.SchemaVersion = definitionSchemaVersion
	if !definitionKeyPattern.MatchString(segment.Key) {
		return badRequest(CodeBadRequestSegmentKeyInvalid, fmt.Sprintf("key %q must match %s", segment.Key, definitionKeyPattern))
	}
	for _, rule := range segment.Rules {
		if err := checkRule(rule.ID, rule.Conditions); err != nil {
			return err
		}
	}
	evalSegment, err := toEval[eval.Segment](segment)
	if err != nil {
		return err
	}
	return evalErrorToAPIError(eval.ValidateSegment(evalSegment))
}

func checkID(kind, id string) error {
	if !definitionIDPattern.MatchString(id) {
		return badRequest(CodeBadRequestDefinitionIDInvalid, fmt.Sprintf("%s id %q must match %s", kind, id, definitionIDPattern))
	}
	return nil
}

// checkRule checks a rule's id and the §2.2 counts of its root group. Nested
// groups are rejected by eval.Validate, so only the root needs counting.
func checkRule(id string, conditions []store.Condition) error {
	if err := checkID("rule", id); err != nil {
		return err
	}
	if len(conditions) > maxGroupChildren {
		return badRequest(CodeBadRequestDefinitionLimitExceeded, fmt.Sprintf("rule %q has %d conditions, max %d", id, len(conditions), maxGroupChildren))
	}
	for _, c := range conditions {
		if len(c.Values) > maxConditionValues {
			return badRequest(CodeBadRequestDefinitionLimitExceeded, fmt.Sprintf("rule %q has a condition with %d values, max %d", id, len(c.Values), maxConditionValues))
		}
	}
	return nil
}

// validateContext checks a context against §6.6.
//
// shortcut: doesn't check the 64 KiB encoded-context cap (the 1 MiB body cap
// bounds it); add it with the binary encoder.
func validateContext(ctx eval.Context) error {
	invalid := func(format string, args ...any) error {
		return badRequest(CodeBadRequestFlagContextInvalid, fmt.Sprintf(format, args...))
	}
	if n := len(ctx.TargetingKey); n == 0 || n > maxTargetingKeyBytes || !utf8.ValidString(ctx.TargetingKey) {
		return invalid("targetingKey must be 1-%d bytes of UTF-8", maxTargetingKeyBytes)
	}
	if len(ctx.Attributes) > maxContextAttributes {
		return invalid("at most %d attributes", maxContextAttributes)
	}
	for name, value := range ctx.Attributes {
		if name == targetingKeyAttribute || !attributeNamePattern.MatchString(name) {
			return invalid("attribute name %q is reserved or doesn't match %s", name, attributeNamePattern)
		}
		if !validAttributeValue(value) {
			return invalid("attribute %q must be a string, number, boolean, or a flat list of one of those kinds", name)
		}
	}
	return nil
}

func validAttributeValue(value any) bool {
	list, isList := value.([]any)
	if !isList {
		return scalarKind(value) != ""
	}
	for _, element := range list {
		if kind := scalarKind(element); kind == "" || kind != scalarKind(list[0]) {
			return false
		}
	}
	return true
}

func scalarKind(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	}
	return ""
}

// flagRuleChanges is the flag routes' audit Details: which rule ids a
// successful write added, changed or removed, or "" when none. before is the
// flag before the write; after is the response body, which holds the flag,
// {"flags": [...]} for a multi-environment create, or neither for a delete.
func flagRuleChanges(before string, after []byte) string {
	var old, cur struct {
		Rules []store.Rule `json:"rules"`
		Flags []store.Flag `json:"flags"`
	}
	_ = json.Unmarshal([]byte(before), &old)
	_ = json.Unmarshal(after, &cur)
	if len(cur.Flags) > 0 {
		cur.Rules = cur.Flags[0].Rules // every created copy has the same rules
	}

	oldRules := make(map[string]string, len(old.Rules))
	for _, rule := range old.Rules {
		b, _ := json.Marshal(rule)
		oldRules[rule.ID] = string(b)
	}
	var changes struct {
		Added   []string `json:"added,omitempty"`
		Changed []string `json:"changed,omitempty"`
		Removed []string `json:"removed,omitempty"`
	}
	for _, rule := range cur.Rules {
		b, _ := json.Marshal(rule)
		prev, existed := oldRules[rule.ID]
		switch {
		case !existed:
			changes.Added = append(changes.Added, rule.ID)
		case prev != string(b):
			changes.Changed = append(changes.Changed, rule.ID)
		}
		delete(oldRules, rule.ID)
	}
	for _, rule := range old.Rules {
		if _, gone := oldRules[rule.ID]; gone {
			changes.Removed = append(changes.Removed, rule.ID)
		}
	}
	if changes.Added == nil && changes.Changed == nil && changes.Removed == nil {
		return ""
	}
	b, _ := json.Marshal(map[string]any{"rules": changes})
	return string(b)
}
