package utils

import "sync/atomic"

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
