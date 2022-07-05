package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

//func TestGetBlockDeviceFromDevnum(t *testing.T) {
//	// test random dev
//	dev, err := GetDeviceFromDevnum(CharDeviceType, 1, 8)
//	assert.Nil(t, err)
//	assert.Equal(t, "/dev/random", dev.Devnode())
//}

func TestGetDevicePath(t *testing.T) {
	devPath, err := GetDevicePath(1, 8)
	fmt.Println(devPath)
	assert.Nil(t, err)
	assert.Equal(t, "/dev/random", devPath)
}
