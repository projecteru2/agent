package utils

import (
	"github.com/cornelk/hashmap"
)

// GroupCAS indicates cas locks which are grouped by keys.
type GroupCAS struct {
	*hashmap.Map[string, struct{}]
}

// NewGroupCAS .
func NewGroupCAS() *GroupCAS {
	return &GroupCAS{
		Map: hashmap.New[string, struct{}](),
	}
}

// Acquire tries to acquire a cas lock.
func (g *GroupCAS) Acquire(key string) (free func(), acquired bool) {
	if _, loaded := g.GetOrInsert(key, struct{}{}); loaded {
		return nil, false
	}

	free = func() {
		g.Del(key)
	}

	return free, true
}
