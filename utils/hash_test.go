package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashBackend(t *testing.T) {
	data := []string{
		"s1",
		"s2",
	}
	backend := NewHashBackends(data)
	assert.Equal(t, backend.Get("param1", 0), "s2")
	assert.Equal(t, backend.Get("param2", 0), "s1")
	assert.Empty(t, NewHashBackends(nil).Get("param1", 0))
}
