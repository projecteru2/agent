package utils

import "github.com/alphadose/haxmap"

// GroupCAS is a set of compare-and-swap locks keyed by string.
type GroupCAS struct {
	*haxmap.Map[string, struct{}]
}

func NewGroupCAS() *GroupCAS {
	return &GroupCAS{
		Map: haxmap.New[string, struct{}](),
	}
}

func (g *GroupCAS) Acquire(key string) (free func(), acquired bool) {
	if _, loaded := g.GetOrSet(key, struct{}{}); loaded {
		return nil, false
	}

	free = func() {
		g.Del(key)
	}

	return free, true
}
