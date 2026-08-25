//go:build !linux

package utils

func GetDevicePath(_, _ uint64) (string, error) {
	return "/dev/random", nil
}
