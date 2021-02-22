package utils

import (
	"sync"
	"sync/atomic"
)

// AtomicBool indicates an atomic boolean instance.
type AtomicBool struct {
	i32 int32
}

// Bool .
func (a *AtomicBool) Bool() bool {
	return atomic.LoadInt32(&a.i32) == 1
}

// Set to true.
func (a *AtomicBool) Set() {
	atomic.StoreInt32(&a.i32, 1)
}

// Unset to false.
func (a *AtomicBool) Unset() {
	atomic.StoreInt32(&a.i32, 0)
}

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
