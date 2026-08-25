package core

import (
	"sync"
	"time"
)

type statusEntry struct {
	value    string
	expireAt time.Time
}

type statusCache struct {
	mu      sync.RWMutex
	entries map[string]statusEntry
}

func newStatusCache() *statusCache {
	return &statusCache{entries: map[string]statusEntry{}}
}

func (c *statusCache) Get(key string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().Before(entry.expireAt) {
		return entry.value, true
	}

	c.Delete(key)
	return "", false
}

func (c *statusCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = statusEntry{value: value, expireAt: time.Now().Add(ttl)}
}

func (c *statusCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}
