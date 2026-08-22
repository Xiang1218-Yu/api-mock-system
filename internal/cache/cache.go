// Package cache is an in-process TTL cache standing in for Redis during local
// development. It is concurrency-safe and generic over [string, any]; callers
// type-assert on retrieval. When Redis is wired in later, this package is the
// single swap-out point.
package cache

import (
	"strings"
	"sync"
	"time"
)

// entry holds a value and its expiration. Zero time means no expiry.
type entry struct {
	value  any
	expiry time.Time
}

// Cache is a thread-safe map of string keys to TTL-bounded values.
type Cache struct {
	mu   sync.RWMutex
	data map[string]entry
	now  func() time.Time
}

// New returns an empty Cache. The optional clock lets tests control time.
func New() *Cache {
	return &Cache{data: make(map[string]entry), now: time.Now}
}

// Set stores value under key with the given TTL. A zero TTL means "never expire".
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = c.now().Add(ttl)
	}
	c.mu.Lock()
	c.data[key] = entry{value: value, expiry: exp}
	c.mu.Unlock()
}

// Get returns the value for key and whether it was present and unexpired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.expiry.IsZero() && !c.now().Before(e.expiry) {
		c.mu.Lock()
		if current, exists := c.data[key]; exists && current.expiry.Equal(e.expiry) {
			delete(c.data, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return e.value, true
}

// Delete removes a single key. Missing keys are a no-op.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}

// DeleteByPrefix removes every key that starts with prefix. Returns the count
// removed. A non-empty prefix is required to avoid accidentally clearing the
// whole cache through an empty-string key.
func (c *Cache) DeleteByPrefix(prefix string) int {
	if prefix == "" {
		return 0
	}
	now := c.now()
	removed := 0
	c.mu.Lock()
	for k, e := range c.data {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		// Skip entries that are merely expired — Purge owns those, and counting
		// them here would over-report. (They'll be swept on next access anyway.)
		if !e.expiry.IsZero() && now.After(e.expiry) {
			continue
		}
		delete(c.data, k)
		removed++
	}
	c.mu.Unlock()
	return removed
}

// Purge removes expired entries. Called by the cleanup cron.
func (c *Cache) Purge() int {
	now := c.now()
	removed := 0
	c.mu.Lock()
	for k, e := range c.data {
		if !e.expiry.IsZero() && !now.Before(e.expiry) {
			delete(c.data, k)
			removed++
		}
	}
	c.mu.Unlock()
	return removed
}

// Len returns the current number of live entries (expired entries may be
// counted until next access; this is best-effort and not transactional).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := c.now()
	count := 0
	for _, e := range c.data {
		if e.expiry.IsZero() || now.Before(e.expiry) {
			count++
		}
	}
	return count
}
