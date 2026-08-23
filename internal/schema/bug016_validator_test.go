package schema

import (
	"strings"
	"testing"
)

func TestValidateNestedObjectAndArray(t *testing.T) {
	value := map[string]any{
		"name":  "Ada",
		"roles": []any{"admin", "viewer"},
		"meta":  map[string]any{"active": true},
	}
	err := Validate(value, map[string]any{
		"type":     "object",
		"required": []any{"name", "roles"},
		"properties": map[string]any{
			"name":  map[string]any{"type": "string", "minLength": 3},
			"roles": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"enum": []any{"admin", "viewer"}}},
			"meta":  map[string]any{"type": "object", "properties": map[string]any{"active": map[string]any{"type": "boolean"}}},
		},
	})
	if err != nil {
		t.Fatalf("expected valid value, got %v", err)
	}
}

func TestValidateCollectsViolations(t *testing.T) {
	err := Validate(map[string]any{"age": 3, "role": "guest"}, map[string]any{
		"type":     "object",
		"required": []any{"email"},
		"properties": map[string]any{
			"age":  map[string]any{"type": "integer", "minimum": 10},
			"role": map[string]any{"enum": []any{"admin", "viewer"}},
		},
	})
	if err == nil {
		t.Fatal("expected validation failure")
	}
	var validationErr *Error
	if !strings.Contains(err.Error(), "email") || !strings.Contains(err.Error(), "age") || !strings.Contains(err.Error(), "role") {
		t.Fatalf("expected all violations, got %v", err)
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected keyword in error, got %v", err)
	}
	validationErr = err.(*Error)
	if len(validationErr.Details()) != 3 {
		t.Fatalf("expected three violation details, got %d", len(validationErr.Details()))
	}
}

func TestValidateOneOfAndFormats(t *testing.T) {
	if err := Validate("user@example.com", map[string]any{"oneOf": []any{
		map[string]any{"type": "string", "format": "email"},
		map[string]any{"type": "string", "format": "uuid"},
	}}); err != nil {
		t.Fatalf("expected email branch to match: %v", err)
	}
	if err := Validate("not-an-email", map[string]any{"type": "string", "format": "email"}); err == nil {
		t.Fatal("expected invalid email")
	}
}

func TestDecodeJSONBody(t *testing.T) {
	if _, present, err := DecodeJSONBody(" "); present || err != nil {
		t.Fatalf("expected omitted body, present=%v err=%v", present, err)
	}
	value, present, err := DecodeJSONBody(`{"count":2,"items":[true]}`)
	if err != nil || !present {
		t.Fatalf("expected decoded body, present=%v err=%v", present, err)
	}
	body := value.(map[string]any)
	if _, ok := body["count"].(int64); !ok {
		t.Fatalf("expected integer JSON number, got %T", body["count"])
	}
	if _, _, err := DecodeJSONBody(`{"ok":true} {"extra":true}`); err == nil {
		t.Fatal("expected trailing JSON value to fail")
	}
}
