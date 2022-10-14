// Package utils for pool
package utils

import (
	"github.com/panjf2000/ants/v2"
)

// Pool indicate global Pool
var Pool *ants.Pool

// NewPool init global goroutine pool
func NewPool(concurrency int) (err error) {
	Pool, err = ants.NewPool(concurrency, ants.WithNonblocking(true))
	return
}
