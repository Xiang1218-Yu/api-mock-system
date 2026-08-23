package mockengine

import "testing"

// TestSeedFromRequestKeysOnQueryAndBody locks the generation-side isolation:
// two requests to the same API that differ only by query params or JSON body
// must seed different random sources, so generated mock data diverges rather
// than echoing the first request's value. Before the fix the seed ignored
// query and body, collapsing distinct requests onto one deterministic value.
func TestSeedFromRequestKeysOnQueryAndBody(t *testing.T) {
	const apiID = "api-1"
	method, path := "GET", "/users"

	base := SeedFromRequest(apiID, method, path, "", "")
	for _, c := range []struct {
		name, query, body string
	}{
		{"query a", "page=1", ""},
		{"query b", "page=2", ""},
		{"body a", "", `{"id":1}`},
		{"body b", "", `{"id":2}`},
		{"both", "page=1", `{"id":1}`},
	} {
		got := SeedFromRequest(apiID, method, path, c.query, c.body)
		if got == base {
			t.Errorf("%s: seed == base seed (%d); query/body not folded in", c.name, base)
		}
	}
}

// TestSeedFromRequestIsDeterministic locks the consistency half of the
// contract: the same request signature always yields the same seed, so the
// same call returns the same mock data without an external store.
func TestSeedFromRequestIsDeterministic(t *testing.T) {
	a := SeedFromRequest("api-1", "GET", "/users", "page=1", `{"id":1}`)
	b := SeedFromRequest("api-1", "GET", "/users", "page=1", `{"id":1}`)
	if a != b {
		t.Fatalf("identical requests seeded differently: %d vs %d", a, b)
	}
}

// TestNewSeededDivergesByQueryAndBody ties the seed back to observable output:
// engines seeded from requests that differ only by query or body must produce
// different generated values for the same schema.
func TestNewSeededDivergesByQueryAndBody(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"n": map[string]any{"type": "integer", "minimum": 0, "maximum": 100000},
		},
	}
	a := NewSeeded("api-1", "GET", "/users", "page=1", "").Generate(schema)
	b := NewSeeded("api-1", "GET", "/users", "page=2", "").Generate(schema)
	if eq(a, b) {
		t.Fatalf("different query produced identical generated body: %v", a)
	}
	c := NewSeeded("api-1", "GET", "/users", "", `{"id":1}`).Generate(schema)
	d := NewSeeded("api-1", "GET", "/users", "", `{"id":2}`).Generate(schema)
	if eq(c, d) {
		t.Fatalf("different body produced identical generated body: %v", c)
	}
}

func eq(a, b any) bool {
	am, ok1 := a.(map[string]any)
	bm, ok2 := b.(map[string]any)
	if !ok1 || !ok2 {
		return a == b
	}
	if len(am) != len(bm) {
		return false
	}
	for k, av := range am {
		if bv, ok := bm[k]; !ok || av != bv {
			return false
		}
	}
	return true
}
