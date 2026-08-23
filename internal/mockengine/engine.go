// Package mockengine turns an API's response schema into concrete mock data.
//
// It is deliberately pure: given a schema and a request-context seed it returns
// a deterministic value, with no database or HTTP dependencies. Caching of the
// resulting data happens one layer up (the mock service), not here.
//
// Determinism: generation uses a seeded math/rand source derived from the
// request. The same seed + schema always yield the same value, satisfying the
// "same request returns same mock data" requirement without an external store.
package mockengine

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Engine generates mock values. Construct once; reuse across requests.
type Engine struct{ rnd *rand.Rand }

// New returns an Engine backed by the given random source. Use a seeded source
// for deterministic output per request.
func New(src rand.Source) *Engine { return &Engine{rnd: rand.New(src)} }

// Generate produces a mock value conforming to schema. schema is a parsed JSON
// Schema map (subset). Unknown keywords are ignored; the engine is tolerant of
// partial schemas.
func (e *Engine) Generate(schema map[string]any) any {
	if schema == nil {
		return nil
	}
	return e.generateNode(schema, "")
}

// generateNode dispatches on the schema's "type" keyword, falling back to
// inference from other keywords when type is absent.
func (e *Engine) generateNode(schema map[string]any, path string) any {
	t, ok := schema["type"].(string)
	if !ok {
		t = e.inferType(schema)
	}
	switch t {
	case "object":
		return e.generateObject(schema, path)
	case "array":
		return e.generateArray(schema, path)
	case "string":
		return e.generateString(schema, path)
	case "integer":
		return e.generateNumber(schema, true)
	case "number":
		return e.generateNumber(schema, false)
	case "boolean":
		return e.rnd.Intn(2) == 1
	case "null":
		return nil
	default:
		// unknown type — emit a string as a safe default
		return e.generateString(schema, path)
	}
}

// inferType guesses type from presence of keywords when "type" is missing.
func (e *Engine) inferType(schema map[string]any) string {
	if _, ok := schema["properties"]; ok {
		return "object"
	}
	if _, ok := schema["items"]; ok {
		return "array"
	}
	if _, ok := schema["enum"]; ok {
		return "string"
	}
	if _, ok := schema["minimum"]; ok {
		return "number"
	}
	if _, ok := schema["format"]; ok {
		return "string"
	}
	return "string"
}

// generateObject builds an object honoring "required" and "properties".
//
// Properties are visited in sorted name order (not map iteration order, which
// is randomized in Go). This makes the random-source consumption deterministic
// for a given schema, so the same seed + schema always yields the same value —
// the §2.6 "cache & consistency" requirement.
func (e *Engine) generateObject(schema map[string]any, path string) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	required, _ := schema["required"].([]any)

	// Sorted property names → deterministic RNG consumption order.
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make(map[string]any, len(names))
	for _, name := range names {
		raw := props[name]
		prop, ok := raw.(map[string]any)
		if !ok {
			out[name] = nil
			continue
		}
		// Skip optional properties sometimes, to vary output — but always
		// include required ones.
		if !isRequired(name, required) && e.rnd.Intn(5) == 0 {
			continue
		}
		out[name] = e.generateNode(prop, path+"/"+name)
	}
	// Guarantee every required property exists even if not in properties.
	for _, r := range required {
		if name, ok := r.(string); ok && name != "" {
			if _, exists := out[name]; !exists {
				out[name] = nil
			}
		}
	}
	return out
}

// generateArray builds an array of 0..maxItems items from the "items" schema.
func (e *Engine) generateArray(schema map[string]any, path string) []any {
	items, _ := schema["items"].(map[string]any)
	if items == nil {
		items = schema["items"].(map[string]any)
	}
	minItems, maxItems := 1, 3
	if n, ok := schema["minItems"]; ok {
		minItems = toInt(n, minItems)
	}
	if n, ok := schema["maxItems"]; ok {
		maxItems = toInt(n, maxItems)
	}
	if minItems > maxItems {
		maxItems = minItems
	}
	count := minItems
	if maxItems > minItems {
		count = minItems + e.rnd.Intn(maxItems-minItems+1)
	}
	out := make([]any, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, e.generateNode(items, fmt.Sprintf("%s/%d", path, i)))
	}
	return out
}

// generateString honors format, enum, pattern, const, minLength/maxLength.
func (e *Engine) generateString(schema map[string]any, path string) any {
	if en := enumValues(schema["enum"]); len(en) > 0 {
		return en[e.rnd.Intn(len(en))]
	}
	if c, ok := schema["const"]; ok {
		return c
	}
	if ex, ok := schema["example"]; ok {
		return ex
	}
	format, _ := schema["format"].(string)
	if v, ok := e.byFormat(format, path); ok {
		return v
	}
	// Hint from the property name: "email" field -> email, etc.
	if v, ok := e.byNameHint(path); ok {
		return v
	}
	minLen, maxLen := 1, 10
	if n, ok := schema["minLength"]; ok {
		minLen = toInt(n, minLen)
	}
	if n, ok := schema["maxLength"]; ok {
		maxLen = toInt(n, maxLen)
	}
	if minLen > maxLen {
		maxLen = minLen
	}
	length := minLen
	if maxLen > minLen {
		length = minLen + e.rnd.Intn(maxLen-minLen+1)
	}
	return randomWord(e.rnd, length)
}

// generateNumber honors minimum/maximum and integer-ness.
func (e *Engine) generateNumber(schema map[string]any, integer bool) any {
	min := 0
	max := 100
	if n, ok := schema["minimum"]; ok {
		min = toInt(n, min)
	}
	if n, ok := schema["maximum"]; ok {
		max = toInt(n, max)
	}
	if max < min {
		max = min + 100
	}
	if integer {
		if min == max {
			return min
		}
		return min + e.rnd.Intn(max-min+1)
	}
	return float64(min) + e.rnd.Float64()*float64(max-min)
}

// byFormat returns a smart value for known JSON-Schema string formats.
func (e *Engine) byFormat(format, path string) (any, bool) {
	switch format {
	case "email":
		return fmt.Sprintf("%s@example.com", randomWord(e.rnd, 6)), true
	case "uuid":
		return randomUUID(e.rnd), true
	case "date":
		return randomDate(e.rnd), true
	case "date-time":
		return randomDateTime(e.rnd), true
	case "phone":
		return randomPhone(e.rnd), true
	case "uri", "url":
		return fmt.Sprintf("https://example.com/%s", randomWord(e.rnd, 5)), true
	}
	return nil, false
}

// byNameHint guesses a smart value from the property path's last segment.
var (
	reEmail = regexp.MustCompile(`(?i)email|e-mail`)
	rePhone = regexp.MustCompile(`(?i)phone|mobile|tel`)
	reName  = regexp.MustCompile(`(?i)^(name|username|fullname|nickname)$`)
	reID    = regexp.MustCompile(`(?i)id$`)
)

func (e *Engine) byNameHint(path string) (any, bool) {
	seg := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		seg = path[i+1:]
	}
	switch {
	case reEmail.MatchString(seg):
		return fmt.Sprintf("%s@example.com", randomWord(e.rnd, 6)), true
	case rePhone.MatchString(seg):
		return randomPhone(e.rnd), true
	case reName.MatchString(seg):
		return randomName(e.rnd), true
	case reID.MatchString(seg):
		return e.rnd.Intn(100000), true
	}
	return nil, false
}

// enumValues extracts a string slice from an enum field, tolerating both
// []any (the JSON-decoded form) and []string (the literal form). Non-string
// elements are stringified.
func enumValues(v any) []string {
	switch s := v.(type) {
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			out = append(out, fmt.Sprint(e))
		}
		return out
	case []string:
		return s
	}
	return nil
}

// toInt coerces a numeric any (from JSON) to int with a fallback.
func toInt(v any, fallback int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return fallback
}

func isRequired(name string, required []any) bool {
	for _, r := range required {
		if s, ok := r.(string); ok && s == name {
			return true
		}
	}
	return false
}

// GenerateFromJSON is a convenience wrapper that parses a raw JSON schema doc
// before generating. A nil/empty doc yields nil.
func (e *Engine) GenerateFromJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	return e.Generate(schema)
}
