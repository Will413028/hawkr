package cache

import (
	"sync"
	"time"
)

type entry struct {
	value     interface{}
	expiresAt time.Time
}

// MemoryCache is a simple in-memory cache with TTL support.
type MemoryCache struct {
	data sync.Map
	ttl  time.Duration
}

// New creates a MemoryCache with the given TTL.
func New(ttl time.Duration) *MemoryCache {
	return &MemoryCache{ttl: ttl}
}

// Get returns the cached value or nil if not found / expired.
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	raw, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	e := raw.(*entry)
	if time.Now().After(e.expiresAt) {
		c.data.Delete(key)
		return nil, false
	}
	return e.value, true
}

// Set stores a value with the default TTL.
func (c *MemoryCache) Set(key string, value interface{}) {
	c.data.Store(key, &entry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Delete removes a key from the cache.
func (c *MemoryCache) Delete(key string) {
	c.data.Delete(key)
}
