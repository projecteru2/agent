//go:build linux

package utils

import (
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/projecteru2/agent/common"
)

const blkDevDir = "/dev/"

func GetDevicePath(major, minor uint64) (string, error) {
	entries, err := os.ReadDir(blkDevDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		fi, err := entry.Info()
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeDevice != os.ModeDevice {
			continue
		}
		stat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return "", common.ErrSyscallFailed
		}
		if uint64(unix.Major(stat.Rdev)) == major && uint64(unix.Minor(stat.Rdev)) == minor {
			return path.Join(blkDevDir, fi.Name()), nil
		}
	}
	return "", common.ErrDevNotFound
}
