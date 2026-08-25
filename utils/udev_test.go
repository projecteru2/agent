package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevicePath(t *testing.T) {
	devPath, err := GetDevicePath(1, 8)
	fmt.Println(devPath)
	assert.Nil(t, err)
	assert.Equal(t, "/dev/random", devPath)
}
