package mockengine

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

func TestGenerateObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":    map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
			"name":  map[string]any{"type": "string"},
			"email": map[string]any{"type": "string", "format": "email"},
			"role":  map[string]any{"type": "string", "enum": []any{"admin", "user"}},
		},
		"required": []any{"id", "name", "email", "role"},
	}
	e := New(rand.NewSource(1))
	v := e.Generate(schema)
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", v)
	}
	for _, k := range []string{"id", "name", "email"} {
		if _, ok := m[k]; !ok {
			t.Errorf("required field %q missing", k)
		}
	}
	id, ok := m["id"].(int)
	if !ok || id < 1 || id > 100 {
		t.Errorf("id out of range or wrong type: %v", m["id"])
	}
	email, _ := m["email"].(string)
	if !strings.HasSuffix(email, "@example.com") {
		t.Errorf("email not well-formed: %q", email)
	}
	role, _ := m["role"].(string)
	if role != "admin" && role != "user" {
		t.Errorf("role not from enum: %q", role)
	}
}

func TestGenerateArray(t *testing.T) {
	schema := map[string]any{
		"type":     "array",
		"minItems": 3,
		"maxItems": 3,
		"items":    map[string]any{"type": "integer", "minimum": 0, "maximum": 9},
	}
	e := New(rand.NewSource(2))
	v := e.Generate(schema)
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected array, got %T", v)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 items, got %d", len(arr))
	}
	for i, item := range arr {
		n, ok := item.(int)
		if !ok || n < 0 || n > 9 {
			t.Errorf("item %d out of range: %v", i, item)
		}
	}
}

// TestDeterminism proves the consistency requirement: same seed+schema → same data.
func TestDeterminism(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"email": map[string]any{"type": "string", "format": "email"},
		},
	}
	a := NewSeeded("api-1", "GET", "/users/1", "", "").Generate(schema)
	b := NewSeeded("api-1", "GET", "/users/1", "", "").Generate(schema)
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) {
		t.Errorf("determinism violated:\n %s\n %s", aj, bj)
	}
	// Different request should usually diverge.
	c := NewSeeded("api-1", "GET", "/users/2", "", "").Generate(schema)
	cj, _ := json.Marshal(c)
	if string(cj) == string(aj) {
		t.Errorf("different request produced identical data (unexpected)")
	}
}

func TestInferType(t *testing.T) {
	e := New(rand.NewSource(1))
	if e.inferType(map[string]any{"properties": map[string]any{}}) != "object" {
		t.Error("infer object failed")
	}
	if e.inferType(map[string]any{"items": map[string]any{}}) != "array" {
		t.Error("infer array failed")
	}
}

func TestNameHint(t *testing.T) {
	e := New(rand.NewSource(1))
	v := e.Generate(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"email": map[string]any{"type": "string"},
			"phone": map[string]any{"type": "string"},
			"name":  map[string]any{"type": "string"},
		},
		"required": []any{"email", "phone", "name"},
	})
	m := v.(map[string]any)
	if email, _ := m["email"].(string); !strings.HasSuffix(email, "@example.com") {
		t.Errorf("email hint failed: %q", email)
	}
	if phone, _ := m["phone"].(string); phone[:3] != "+86" {
		t.Errorf("phone hint failed: %q", phone)
	}
	if name, _ := m["name"].(string); name == "" {
		t.Error("name hint failed: empty")
	}
}
