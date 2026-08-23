package cache

import (
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := New()
	c.Set("k", "v", 0)
	v, ok := c.Get("k")
	if !ok || v != "v" {
		t.Errorf("Get = %v, %v", v, ok)
	}
}

func TestExpiry(t *testing.T) {
	c := New()
	now := time.Now()
	clock := now
	c.now = func() time.Time { return clock }

	c.Set("k", "v", 10*time.Millisecond)
	clock = now.Add(20 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Error("expected expiry")
	}
}

func TestDelete(t *testing.T) {
	c := New()
	c.Set("k", "v", 0)
	c.Delete("k")
	if _, ok := c.Get("k"); ok {
		t.Error("expected deleted")
	}
}

func TestPurgeRemovesOnlyExpired(t *testing.T) {
	c := New()
	now := time.Now()
	clock := now
	c.now = func() time.Time { return clock }

	c.Set("live", 1, 0)
	c.Set("dead", 2, 10*time.Millisecond)
	clock = now.Add(20 * time.Millisecond)

	if n := c.Purge(); n != 1 {
		t.Errorf("purge removed %d, want 1", n)
	}
	if _, ok := c.Get("live"); !ok {
		t.Error("live entry was purged")
	}
}

func TestDeleteByPrefix(t *testing.T) {
	c := New()
	c.Set("mock:api-a|GET", 1, 0)
	c.Set("mock:api-a|POST", 2, 0)
	c.Set("mock:api-b|GET", 3, 0)
	c.Set("other", 4, 0)

	n := c.DeleteByPrefix("mock:api-a|")
	if n != 2 {
		t.Errorf("removed %d, want 2", n)
	}
	if _, ok := c.Get("mock:api-a|GET"); ok {
		t.Error("prefix entry still present")
	}
	if _, ok := c.Get("mock:api-b|GET"); !ok {
		t.Error("non-matching prefix was removed")
	}
	if _, ok := c.Get("other"); !ok {
		t.Error("unrelated entry was removed")
	}
}

func TestDeleteByPrefixEmptyNoOp(t *testing.T) {
	c := New()
	c.Set("k", "v", 0)
	if n := c.DeleteByPrefix(""); n != 0 {
		t.Errorf("empty prefix removed %d, want 0", n)
	}
	if _, ok := c.Get("k"); !ok {
		t.Error("empty prefix deleted entries")
	}
}
