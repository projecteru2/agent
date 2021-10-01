package utils

import (
	"sync"
)

// GroupCAS indicates cas locks which are grouped by keys.
type GroupCAS struct {
	groups sync.Map
}

// Acquire tries to acquire a cas lock.
func (c *GroupCAS) Acquire(key string) (free func(), acuired bool) {
	_, loaded := c.groups.LoadOrStore(key, struct{}{})
	if loaded {
		return
	}

	free = func() {
		c.groups.Delete(key)
	}

	return free, true
}
