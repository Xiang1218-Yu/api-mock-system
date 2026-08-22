package cache

import (
	"testing"
	"time"
)

func TestExpiryBoundaryIsNotCountedByConcurrentCleanup(t *testing.T) {
	c := New()
	now := time.Now()
	clock := now
	c.now = func() time.Time { return clock }
	c.Set("mock:api|GET", "value", time.Second)
	clock = now.Add(time.Second)

	done := make(chan int, 1)
	go func() {
		done <- c.DeleteByPrefix("mock:api|")
	}()
	if removed := <-done; removed != 0 {
		t.Fatalf("expired entry was counted as deleted: %d", removed)
	}
	if c.Len() != 0 {
		t.Fatal("expired entry remained live at the exact boundary")
	}
}
