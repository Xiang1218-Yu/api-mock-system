package mockengine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// seededEngine returns a deterministic engine so test output is stable.
func seededEngine(t *testing.T, apiID string) *Engine {
	t.Helper()
	return NewSeeded(apiID, "GET", "/x", "", "")
}

// asMap re-decodes generated JSON so element types are plain map[string]any /
// []any, matching what a client receives. Comparing on the re-decoded form
// guards against the named-type-vs-map[string]any mismatch that caused the
// original crash.
func asMap(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal generated value: %v", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal generated value: %v", err)
	}
	return out
}

func TestGenerate_ArraySchema(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":   map[string]any{"type": "integer"},
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"id", "name"},
		},
		"minItems": 2,
		"maxItems": 2,
	}
	got := asMap(t, seededEngine(t, "api-array").Generate(schema))

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	for i, el := range arr {
		obj, ok := el.(map[string]any)
		if !ok {
			t.Fatalf("element %d: expected map, got %T", i, el)
		}
		if _, ok := obj["id"]; !ok {
			t.Errorf("element %d: missing required field id", i)
		}
		if _, ok := obj["name"]; !ok {
			t.Errorf("element %d: missing required field name", i)
		}
	}
}

func TestGenerate_ArrayShapeMatchesJSON(t *testing.T) {
	// A declared array response must serialize as a JSON array, not an object.
	schema := map[string]any{
		"type":     "array",
		"items":    map[string]any{"type": "integer"},
		"minItems": 3,
		"maxItems": 3,
	}
	b, err := json.Marshal(seededEngine(t, "api-arr2").Generate(schema))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 || b[0] != '[' {
		t.Fatalf("array response must serialize as JSON array, got %q", b)
	}
	var ints []int
	if err := json.Unmarshal(b, &ints); err != nil {
		t.Fatalf("not a JSON array of ints: %v (body=%s)", err, b)
	}
	if len(ints) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(ints))
	}
}

func TestGenerate_MalformedArraySchemasDoNotCrash(t *testing.T) {
	// Each of these previously panicked generateArray's unchecked assertion.
	cases := map[string]map[string]any{
		"no-items":        {"type": "array", "minItems": 2, "maxItems": 3},
		"tuple-items":     {"type": "array", "items": []any{map[string]any{"type": "integer"}, map[string]any{"type": "string"}}},
		"items-false":     {"type": "array", "items": false},
		"items-null":      {"type": "array", "items": nil},
		"items-string":    {"type": "array", "items": "not-a-schema"},
		"items-as-number": {"type": "array", "items": 42},
	}
	e := seededEngine(t, "api-malformed")
	for name, schema := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Generate panicked for %s: %v", name, r)
				}
			}()
			got := e.Generate(schema)
			if _, ok := got.([]any); !ok {
				t.Fatalf("array schema %s must yield []any, got %T (%v)", name, got, got)
			}
		})
	}
}

func TestGenerate_ObjectSchemaStillWorks(t *testing.T) {
	// Regression: the service previously unwrapped array "items" into the root;
	// make sure a plain object schema round-trips as an object.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":    map[string]any{"type": "integer"},
			"email": map[string]any{"type": "string", "format": "email"},
		},
		"required": []any{"id", "email"},
	}
	got := asMap(t, seededEngine(t, "api-obj").Generate(schema))
	obj, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	if obj["email"] == nil {
		t.Errorf("missing required email")
	}
}

func TestGenerate_NestedArrayInObject(t *testing.T) {
	// Array-of-objects nested inside an object — a common real-world shape.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":     "array",
				"items":    map[string]any{"type": "string"},
				"minItems": 2,
				"maxItems": 2,
			},
		},
	}
	got := asMap(t, seededEngine(t, "api-nested").Generate(schema))
	obj := got.(map[string]any)
	tags, ok := obj["tags"].([]any)
	if !ok {
		t.Fatalf("tags: expected []any, got %T", obj["tags"])
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if reflect.TypeOf(tags[0]).Kind() != reflect.String {
		t.Errorf("tag element: expected string, got %T", tags[0])
	}
}

func TestGenerate_NilSchema(t *testing.T) {
	// A nil schema is a valid input (Generate's documented contract).
	if got := seededEngine(t, "api-nil").Generate(nil); got != nil {
		t.Errorf("nil schema should yield nil, got %v", got)
	}
}
