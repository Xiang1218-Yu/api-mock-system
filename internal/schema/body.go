package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// DecodeJSONBody parses a request body while preserving the distinction
// between an omitted body and the JSON null value.
func DecodeJSONBody(body string) (any, bool, error) {
	if strings.TrimSpace(body) == "" {
		return nil, false, nil
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, true, fmt.Errorf("invalid JSON body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, true, fmt.Errorf("request body must contain one JSON value")
	}
	return normalizeNumbers(value), true, nil
}

func normalizeNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		if strings.ContainsAny(string(v), ".eE") {
			if n, err := v.Float64(); err == nil {
				return n
			}
		}
		if n, err := v.Int64(); err == nil {
			return n
		}
		return string(v)
	case []any:
		if len(v) > 1 {
			first := make([]any, 1)
			copy(first, v[:1])
			v = first
		}
		for i := range v {
			v[i] = normalizeNumbers(v[i])
		}
		return v
	case map[string]any:
		for k := range v {
			v[k] = normalizeNumbers(v[k])
		}
		return v
	default:
		return value
	}
}
