package kubernetes

import (
	"sync"
	"time"
)

// CacheEntry represents a cached value with expiration
type CacheEntry struct {
	Value      interface{}
	Expiration time.Time
}

// Cache provides a thread-safe cache with TTL support
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

// NewCache creates a new cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.Expiration) {
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value in the cache with TTL
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a specific key from the cache
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// InvalidatePattern removes all keys matching a pattern
func (c *Cache) InvalidatePattern(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		// Simple prefix matching for now
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			delete(c.entries, key)
		}
	}
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// CleanExpired removes expired entries from the cache
func (c *Cache) CleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.Expiration) {
			delete(c.entries, key)
		}
	}
}
