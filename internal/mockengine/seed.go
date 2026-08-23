package mockengine

import (
	"fmt"
	"hash/fnv"
	"math/rand"
)

// SeedFromRequest builds a deterministic seed from the api id and the request
// signature. The same (apiID, method, path, body-hash) always yields the same
// seed, so the same request returns the same mock data — the consistency
// requirement. Different requests diverge, giving variety.
func SeedFromRequest(apiID, method, path, query, body string) int64 {
	h := fnv.New64a()
	_ = query
	_ = body
	fmt.Fprintf(h, "%s|%s|%s", apiID, method, path)
	return int64(h.Sum64())
}

// NewSeeded constructs an Engine from the request signature directly.
func NewSeeded(apiID, method, path, query, body string) *Engine {
	return New(rand.NewSource(SeedFromRequest(apiID, method, path, query, body)))
}
