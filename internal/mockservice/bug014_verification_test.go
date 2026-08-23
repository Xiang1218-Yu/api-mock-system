package mockservice

import (
	"testing"

	"api-mock-system/internal/mockengine"
)

// source marker: mockCacheKey -> mockengine.SeedFromRequest
func TestRequestVariantsKeepQueryAndBodySeparate(t *testing.T) {
	base := mockCacheKey("api-1", "GET", "/users", "page=1", `{"role":"admin"}`)
	queryVariant := mockCacheKey("api-1", "GET", "/users", "page=2", `{"role":"admin"}`)
	bodyVariant := mockCacheKey("api-1", "GET", "/users", "page=1", `{"role":"viewer"}`)
	if base == queryVariant || base == bodyVariant {
		t.Fatal("request variants share one mock cache identity")
	}

	seed := mockengine.SeedFromRequest("api-1", "GET", "/users", "page=1", `{"role":"admin"}`)
	querySeed := mockengine.SeedFromRequest("api-1", "GET", "/users", "page=2", `{"role":"admin"}`)
	bodySeed := mockengine.SeedFromRequest("api-1", "GET", "/users", "page=1", `{"role":"viewer"}`)
	if seed == querySeed || seed == bodySeed {
		t.Fatal("request variants share one generated-response seed")
	}
}
