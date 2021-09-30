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
	assert.EqualValues(t, backend.Len(), 2)
	// a certain string will always get a certain hash
	assert.Equal(t, backend.Get("param1", 0), "s2")
	assert.Equal(t, backend.Get("param2", 0), "s1")
}
