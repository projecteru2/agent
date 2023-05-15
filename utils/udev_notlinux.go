//go:build !linux
// +build !linux

package utils

func GetDevicePath(uint64, uint64) (devPath string, err error) {
	return "/dev/random", nil
}
