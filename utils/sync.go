package utils

import (
	"github.com/alphadose/haxmap"
)

// GroupCAS indicates cas locks which are grouped by keys.
type GroupCAS struct {
	*haxmap.Map[string, struct{}]
}

func NewGroupCAS() *GroupCAS {
	return &GroupCAS{
		Map: haxmap.New[string, struct{}](),
	}
}

// Acquire tries to acquire a cas lock.
func (g *GroupCAS) Acquire(key string) (free func(), acquired bool) {
	if _, loaded := g.GetOrSet(key, struct{}{}); loaded {
		return nil, false
	}

	free = func() {
		g.Del(key)
	}

	return free, true
}
