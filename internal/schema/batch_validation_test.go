package schema

import (
	"strings"
	"testing"
)

// TestBatchValidationCollectsAllViolations covers the three fixes that together
// make a batch (array) request report every violation instead of passing it
// through or surfacing only the first error:
//   - body.go normalizeNumbers no longer truncates arrays to their first item
//   - validator.go validateArray now visits every item, not just index 0
//   - (request_validation.go no longer keeps only the first violation)
func TestBatchValidationCollectsAllViolations(t *testing.T) {
	body := `[{"name":"ok","age":1},{"name":123,"age":"x"},{"age":2}]`
	value, present, err := DecodeJSONBody(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !present {
		t.Fatal("body not present")
	}
	arr, ok := value.([]any)
	if !ok {
		t.Fatalf("decoded value is %T, want array", value)
	}
	if len(arr) != 3 {
		t.Fatalf("array truncated after decode: got %d items, want 3", len(arr))
	}

	schemaMap := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "integer"},
			},
			"required": []string{"name"},
		},
	}
	err = Validate(value, schemaMap)
	if err == nil {
		t.Fatal("expected validation error, got nil (batch passed through)")
	}
	var ve *Error
	if !asError(err, &ve) {
		t.Fatalf("expected *schema.Error, got %T", err)
	}
	paths := map[string]bool{}
	for _, v := range ve.Violations {
		paths[v.Path] = true
	}
	// Expect violations at index 1 (name type, age type) and index 2 (name required).
	expected := []string{"$[1].name", "$[1].age", "$[2].name"}
	for _, p := range expected {
		if !paths[p] {
			t.Errorf("missing violation at %s; got %#v", p, ve.Violations)
		}
	}
	if len(ve.Violations) < len(expected) {
		t.Errorf("expected at least %d violations, got %d", len(expected), len(ve.Violations))
	}
	// Error() must surface every path, not just the first.
	msg := ve.Error()
	for _, p := range expected {
		if !strings.Contains(msg, p) {
			t.Errorf("error message missing %s: %s", p, msg)
		}
	}
}

func asError(err error, target **Error) bool {
	if e, ok := err.(*Error); ok {
		*target = e
		return true
	}
	return false
}
