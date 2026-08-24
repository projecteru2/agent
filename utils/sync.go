package utils

import "sync"

// GroupCAS is a set of compare-and-swap locks keyed by string.
type GroupCAS struct {
	keys sync.Map
}

func NewGroupCAS() *GroupCAS {
	return &GroupCAS{}
}

func (g *GroupCAS) Acquire(key string) (free func(), acquired bool) {
	if _, loaded := g.keys.LoadOrStore(key, struct{}{}); loaded {
		return nil, false
	}
	return func() { g.keys.Delete(key) }, true
}
