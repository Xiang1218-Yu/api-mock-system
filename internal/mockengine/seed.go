package mockengine

import (
	"fmt"
	"hash/fnv"
	"math/rand"
)

// SeedFromRequest builds a deterministic seed from the api id and the full
// request signature (method, path, query, body). The same signature always
// yields the same seed, so the same request returns the same mock data — the
// consistency requirement. Query and body are folded in so two requests that
// differ only by query params or JSON body diverge, giving the variety and
// isolation the runtime promises.
func SeedFromRequest(apiID, method, path, query, body string) int64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s", apiID, method, path, query, body)
	return int64(h.Sum64())
}

// NewSeeded constructs an Engine from the request signature directly.
func NewSeeded(apiID, method, path, query, body string) *Engine {
	return New(rand.NewSource(SeedFromRequest(apiID, method, path, query, body)))
}
