package corestore

import (
	"sync"
	"time"
)

type item struct {
	key      string
	value    string
	expireAt int64
}

func (i *item) expired() bool {
	if i.expireAt == 0 {
		return false
	}
	return time.Now().Unix() >= i.expireAt
}

// Cache is ... a cache
// it will cleanup the expired items every interval time
type Cache struct {
	interval time.Duration
	items    *sync.Map
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(c.interval)
	for range ticker.C {
		c.cleanExpired()
	}
}

func (c *Cache) cleanExpired() {
	c.items.Range(func(key, value interface{}) bool {
		i, ok := value.(*item)
		if !ok || !i.expired() {
			return true
		}
		c.items.Delete(key)
		return true
	})
}

// Put puts the key-value pair
// will expire after expireAfter seconds
// you can't put a key that never expires
func (c *Cache) Put(key, value string, expireAfter int64) {
	i := &item{
		key:      key,
		value:    value,
		expireAt: time.Now().Unix() + expireAfter,
	}
	c.items.Store(key, i)
}

// Get gets the value of key
// returns value and ok which represents if the key exists
func (c *Cache) Get(key string) (string, bool) {
	v, ok := c.items.Load(key)
	if !ok {
		return "", false
	}

	i, ok := v.(*item)
	if !ok {
		return "", false
	}
	return i.value, true
}

// NewCache returns a Cache
// it will cleanup every interval seconds
func NewCache(interval int) *Cache {
	c := &Cache{
		interval: time.Duration(interval) * time.Second,
		items:    &sync.Map{},
	}
	go c.cleanup()
	return c
}
