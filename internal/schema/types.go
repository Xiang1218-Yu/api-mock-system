package schema

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func schemaTypes(raw any) []string {
	switch v := raw.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

func matchesAnyType(value any, types []string) bool {
	for _, typ := range types {
		switch typ {
		case "object":
			if _, ok := value.(map[string]any); ok {
				return true
			}
		case "array":
			if _, ok := value.([]any); ok {
				return true
			}
		case "string":
			if _, ok := value.(string); ok {
				return true
			}
		case "number":
			if isNumber(value) {
				return true
			}
		case "integer":
			if isInteger(value) {
				return true
			}
		case "boolean":
			if _, ok := value.(bool); ok {
				return true
			}
		case "null":
			if value == nil {
				return true
			}
		}
	}
	return false
}

func isNumber(value any) bool {
	switch value.(type) {
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func isInteger(value any) bool {
	switch v := value.(type) {
	case float64:
		return v == float64(int64(v))
	case float32:
		return v == float32(int64(v))
	default:
		return isNumber(value)
	}
}

func integer(raw any) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), v == float64(int(v))
	case json.Number:
		n, err := strconv.Atoi(string(v))
		return n, err == nil
	default:
		return 0, false
	}
}

func number(raw any) (float64, bool) {
	switch v := raw.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		n, err := strconv.ParseFloat(string(v), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func stringList(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		if items2, ok := raw.([]string); ok {
			return items2
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func containsValue(values []any, wanted any) bool {
	for _, value := range values {
		if sameValue(value, wanted) {
			return true
		}
	}
	return false
}

func sameValue(a, b any) bool {
	aj, aerr := json.Marshal(a)
	bj, berr := json.Marshal(b)
	return aerr == nil && berr == nil && string(aj) == string(bj)
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func looksLikeDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil && strings.Count(value, "-") == 2
}
