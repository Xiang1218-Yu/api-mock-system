package mockservice

import (
	"testing"
)

// TestRequestSignatureKeysOnQueryAndBody locks the isolation contract: two
// calls to the same published API that differ only by query params or JSON
// body must produce distinct override keys and distinct cache keys. Before the
// fix both ignored query/body, so a fixed override for one request leaked into
// another and a generated response was masked by an unrelated cached one.
func TestRequestSignatureKeysOnQueryAndBody(t *testing.T) {
	const apiID = "api-1"
	method, path := "GET", "/users"

	cases := []struct {
		name  string
		query string
		body  string
	}{
		{"empty", "", ""},
		{"query a", "page=1", ""},
		{"query b", "page=2", ""},
		{"body a", "", `{"id":1}`},
		{"body b", "", `{"id":2}`},
		{"both", "page=1", `{"id":1}`},
	}
	seenSig := map[string]string{}
	seenCache := map[string]string{}
	for _, c := range cases {
		sig := requestSignature(method, path, c.query, c.body)
		if prev, dup := seenSig[sig]; dup {
			t.Fatalf("signature collision: %q (prev %q) for %+v", sig, prev, c)
		}
		seenSig[sig] = c.name

		// The override key is exactly the signature — same identity, same slot.
		if got := mockKey(method, path, c.query, c.body); got != sig {
			t.Errorf("mockKey(%s) = %q, want signature %q", c.name, got, sig)
		}

		// The cache key prefixes the signature with the API-scoped purge prefix;
		// the suffix must still distinguish every distinct request.
		cache := mockCacheKey(apiID, method, path, c.query, c.body)
		if prev, dup := seenCache[cache]; dup {
			t.Fatalf("cache key collision: %q (prev %q) for %+v", cache, prev, c)
		}
		seenCache[cache] = c.name
	}
}

// TestMockCacheKeySharesSignatureWithOverrideKey locks the core fix: the cache
// key and the override key are built from the SAME request signature, so a
// fixed override only ever answers its own request and is not masked by (nor
// masks) a generated response cached for a different request.
func TestMockCacheKeySharesSignatureWithOverrideKey(t *testing.T) {
	const apiID = "api-1"
	method, path, query, body := "POST", "/orders", "v=2", `{"sku":"A"}`

	sig := mockKey(method, path, query, body)
	cache := mockCacheKey(apiID, method, path, query, body)

	want := "mock:" + apiID + "|" + sig
	if cache != want {
		t.Fatalf("cache key = %q, want %q (signature-suffixed)", cache, want)
	}
}

// TestMockCacheKeyPurgePrefixStaysAPIScoped locks the secondary invariant: even
// though cache keys now carry the full signature, every key for an API still
// shares the "mock:<apiID>|" prefix, so Invalidate's prefix purge stays scoped
// to one API and never touches another API's cached responses.
func TestMockCacheKeyPurgePrefixStaysAPIScoped(t *testing.T) {
	method, path := "GET", "/users"
	a1 := mockCacheKey("api-1", method, path, "page=1", "")
	a2 := mockCacheKey("api-2", method, path, "page=1", "")

	// Purge for api-1 must not match api-2's key.
	prefix := "mock:api-1|"
	if hasPrefix(a1, prefix) == false {
		t.Fatalf("api-1 key %q missing purge prefix %q", a1, prefix)
	}
	if hasPrefix(a2, prefix) {
		t.Fatalf("api-2 key %q should not fall under api-1 purge prefix %q", a2, prefix)
	}
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
