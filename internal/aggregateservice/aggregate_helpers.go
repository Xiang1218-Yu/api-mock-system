package aggregateservice

import (
	"errors"

	"api-mock-system/internal/aggregaterepo"
)

func validMode(m string) bool {
	switch m {
	case "serial", "parallel", "conditional":
		return true
	}
	return false
}

func defaultTimeout(ms, fallback int) int {
	if ms <= 0 {
		if fallback <= 0 {
			return 3000
		}
		return fallback
	}
	return ms
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func toStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func mapErr(err error) error {
	if errors.Is(err, aggregaterepo.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
