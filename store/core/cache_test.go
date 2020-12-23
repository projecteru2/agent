package corestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	a := assert.New(t)

	var (
		value string
		ok    bool
	)

	cache := NewCache(1)
	cache.Put("testkey", "testvalue", 1)

	value, ok = cache.Get("keynonexists")
	a.False(ok)
	a.Equal(value, "")

	value, ok = cache.Get("testkey")
	a.True(ok)
	a.Equal(value, "testvalue")

	cache.Put("testkey", "testvalue1", 1)
	value, ok = cache.Get("testkey")
	a.True(ok)
	a.Equal(value, "testvalue1")

	time.Sleep(3 * time.Second)
	value, ok = cache.Get("testkey")
	a.False(ok)
	a.Equal(value, "")
}
