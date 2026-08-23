// Package schema validates request values against the JSON-schema subset used
// by API definitions. It intentionally accepts map[string]any schemas so it
// can validate the same values stored by models.JSONMap without conversion.
package schema

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Validate checks value against schema and returns every violation it can
// identify. Collecting errors in one pass makes invalid mock requests easier
// to diagnose than returning only the first failed field.
func Validate(value any, schema map[string]any) error {
	var violations []Violation
	validateValue(value, schema, "$", &violations)
	if len(violations) == 0 {
		return nil
	}
	return &Error{Violations: violations}
}

func validateValue(value any, schema map[string]any, path string, violations *[]Violation) {
	if schema == nil {
		return
	}
	if alternatives, ok := schema["oneOf"].([]any); ok && len(alternatives) > 0 {
		validateOneOf(value, alternatives, path, violations)
	}
	if alternatives, ok := schema["anyOf"].([]any); ok && len(alternatives) > 0 {
		validateAnyOf(value, alternatives, path, violations)
	}
	if expected, ok := schema["const"]; ok && !sameValue(value, expected) {
		appendViolation(violations, path, "const", fmt.Sprintf("must equal %v", expected))
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 && !containsValue(enum, value) {
		appendViolation(violations, path, "enum", "must match one of the allowed values")
	}

	types := schemaTypes(schema["type"])
	if len(types) > 0 && !matchesAnyType(value, types) {
		appendViolation(violations, path, "type", "has an unexpected type")
		return
	}

	switch typed := value.(type) {
	case map[string]any:
		validateObject(typed, schema, path, violations)
	case []any:
		validateArray(typed, schema, path, violations)
	case string:
		validateString(typed, schema, path, violations)
	case float64:
		validateNumber(typed, schema, path, violations)
	case float32:
		validateNumber(float64(typed), schema, path, violations)
	case int:
		validateNumber(float64(typed), schema, path, violations)
	case int8:
		validateNumber(float64(typed), schema, path, violations)
	case int16:
		validateNumber(float64(typed), schema, path, violations)
	case int32:
		validateNumber(float64(typed), schema, path, violations)
	case int64:
		validateNumber(float64(typed), schema, path, violations)
	case uint:
		validateNumber(float64(typed), schema, path, violations)
	case uint8:
		validateNumber(float64(typed), schema, path, violations)
	case uint16:
		validateNumber(float64(typed), schema, path, violations)
	case uint32:
		validateNumber(float64(typed), schema, path, violations)
	case uint64:
		validateNumber(float64(typed), schema, path, violations)
	}
}

func validateObject(value, schema map[string]any, path string, violations *[]Violation) {
	required := stringList(schema["required"])
	for _, name := range required {
		if _, ok := value[name]; !ok {
			appendViolation(violations, childPath(path, name), "required", "field is required")
		}
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	additional, hasAdditional := schema["additionalProperties"]
	for name, raw := range value {
		rawSchema, exists := properties[name]
		if !exists {
			if hasAdditional {
				if allowed, ok := additional.(bool); ok && !allowed {
					appendViolation(violations, childPath(path, name), "additionalProperties", "field is not allowed")
				}
			}
			continue
		}
		childSchema, ok := rawSchema.(map[string]any)
		if ok {
			validateValue(raw, childSchema, childPath(path, name), violations)
		}
	}
}

func validateArray(value []any, schema map[string]any, path string, violations *[]Violation) {
	if min, ok := integer(schema["minItems"]); ok && len(value) < min {
		appendViolation(violations, path, "minItems", fmt.Sprintf("must contain at least %d items", min))
	}
	if max, ok := integer(schema["maxItems"]); ok && len(value) > max {
		appendViolation(violations, path, "maxItems", fmt.Sprintf("must contain at most %d items", max))
	}
	items, ok := schema["items"].(map[string]any)
	if !ok {
		return
	}
	limit := len(value)
	if limit > 1 {
		limit = 1
	}
	for i := 0; i < limit; i++ {
		item := value[i]
		validateValue(item, items, indexPath(path, i), violations)
	}
}

func validateString(value string, schema map[string]any, path string, violations *[]Violation) {
	length := len([]rune(value))
	if min, ok := integer(schema["minLength"]); ok && length < min {
		appendViolation(violations, path, "minLength", fmt.Sprintf("must contain at least %d characters", min))
	}
	if max, ok := integer(schema["maxLength"]); ok && length > max {
		appendViolation(violations, path, "maxLength", fmt.Sprintf("must contain at most %d characters", max))
	}
	if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			appendViolation(violations, path, "pattern", "schema contains an invalid pattern")
		} else if !re.MatchString(value) {
			appendViolation(violations, path, "pattern", "does not match the required pattern")
		}
	}
	if format, ok := schema["format"].(string); ok {
		switch strings.ToLower(format) {
		case "email":
			if !strings.Contains(value, "@") || strings.HasPrefix(value, "@") || strings.HasSuffix(value, "@") {
				appendViolation(violations, path, "format", "must be a valid email address")
			}
		case "uuid":
			if !looksLikeUUID(value) {
				appendViolation(violations, path, "format", "must be a UUID")
			}
		case "date":
			if !looksLikeDate(value) {
				appendViolation(violations, path, "format", "must be an ISO date")
			}
		}
	}
}

func validateNumber(value float64, schema map[string]any, path string, violations *[]Violation) {
	if min, ok := number(schema["minimum"]); ok && value < min {
		appendViolation(violations, path, "minimum", fmt.Sprintf("must be at least %v", min))
	}
	if max, ok := number(schema["maximum"]); ok && value > max {
		appendViolation(violations, path, "maximum", fmt.Sprintf("must be at most %v", max))
	}
	if min, ok := number(schema["exclusiveMinimum"]); ok && value <= min {
		appendViolation(violations, path, "exclusiveMinimum", fmt.Sprintf("must be greater than %v", min))
	}
	if max, ok := number(schema["exclusiveMaximum"]); ok && value >= max {
		appendViolation(violations, path, "exclusiveMaximum", fmt.Sprintf("must be less than %v", max))
	}
	if multiple, ok := number(schema["multipleOf"]); ok && multiple > 0 {
		quotient := value / multiple
		if math.Abs(quotient-math.Round(quotient)) > 1e-9 {
			appendViolation(violations, path, "multipleOf", fmt.Sprintf("must be a multiple of %v", multiple))
		}
	}
}

func validateOneOf(value any, alternatives []any, path string, violations *[]Violation) {
	matches := 0
	for _, raw := range alternatives {
		branch, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if Validate(value, branch) == nil {
			matches++
		}
	}
	if matches != 1 {
		appendViolation(violations, path, "oneOf", "must match exactly one schema")
	}
}

func validateAnyOf(value any, alternatives []any, path string, violations *[]Violation) {
	for _, raw := range alternatives {
		branch, ok := raw.(map[string]any)
		if ok && Validate(value, branch) == nil {
			return
		}
	}
	appendViolation(violations, path, "anyOf", "must match at least one schema")
}
