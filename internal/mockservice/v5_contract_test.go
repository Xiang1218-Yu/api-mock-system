package mockservice

import (
	"context"
	"testing"

	"api-mock-system/internal/cache"
)

func TestInvalidateKeepsAdjacentAPIKeys(t *testing.T) {
	c := cache.New()
	c.Set("mock:api-1|GET|/users", "first", 0)
	c.Set("mock:api-10|GET|/users", "second", 0)
	s := &Service{cache: c}

	s.Invalidate(context.Background(), "api-1")

	if _, ok := c.Get("mock:api-1|GET|/users"); ok {
		t.Fatal("invalidated API cache entry remained")
	}
	if _, ok := c.Get("mock:api-10|GET|/users"); !ok {
		t.Fatal("cache invalidation removed an adjacent API entry")
	}
}
