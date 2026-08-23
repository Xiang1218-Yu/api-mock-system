package schema

import (
	"fmt"
	"strings"
)

// Violation describes one failed schema rule.
type Violation struct {
	Path    string
	Keyword string
	Message string
}

// Error groups all violations found while validating one request value.
type Error struct {
	Violations []Violation
}

func (e *Error) Error() string {
	if e == nil || len(e.Violations) == 0 {
		return "schema validation failed"
	}
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		location := v.Path
		if location == "" {
			location = "$"
		}
		if v.Keyword == "" {
			parts = append(parts, fmt.Sprintf("%s: %s", location, v.Message))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s): %s", location, v.Keyword, v.Message))
	}
	return "schema validation failed: " + strings.Join(parts, "; ")
}

// Details returns a copy suitable for an HTTP error envelope.
func (e *Error) Details() []map[string]string {
	if e == nil {
		return nil
	}
	out := make([]map[string]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		out = append(out, map[string]string{
			"path":    v.Path,
			"keyword": v.Keyword,
			"message": v.Message,
		})
	}
	return out
}

func appendViolation(dst *[]Violation, path, keyword, message string) {
	*dst = append(*dst, Violation{
		Path:    path,
		Keyword: keyword,
		Message: message,
	})
}

func childPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	if strings.HasPrefix(child, "[") {
		return parent + child
	}
	return parent + "." + child
}

func indexPath(parent string, index int) string {
	return fmt.Sprintf("%s[%d]", parent, index)
}
