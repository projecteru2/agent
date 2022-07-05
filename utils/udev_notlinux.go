//go:build !linux
// +build !linux

package utils

func GetDevicePath(major, minor uint64) (devPath string, err error) {
	return "", nil
}
