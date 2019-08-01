package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPrevCheck(t *testing.T) {
	config := &Config{HealthCheckCacheTTL: 1}
	pc := NewPrevCheck(config)
	v, ok := pc.Get("SOMETHING")
	assert.False(t, v)
	assert.False(t, ok)
	pc.Set("ID", false)
	v, ok = pc.Get("ID")
	assert.False(t, v)
	assert.True(t, ok)
	time.Sleep(1 * time.Second)
	v, ok = pc.Get("ID")
	assert.False(t, v)
	assert.False(t, ok)
}
