//go:build linux
// +build linux

package utils

import (
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/projecteru2/agent/common"
)

const (
	blkDevDir = "/dev/"
)

func GetDevicePath(major, minor uint64) (string, error) {
	entries, err := os.ReadDir(blkDevDir)
	if err != nil {
		return "", err
	}
	dev := getDev(major, minor)
	for _, entry := range entries {
		fi, err := entry.Info()
		if err != nil {
			return "", err
		}
		if (fi.Mode() & os.ModeDevice) != os.ModeDevice {
			continue
		}
		stat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return "", common.ErrSyscallFailed
		}
		if stat.Rdev == dev {
			return path.Join(blkDevDir, fi.Name()), nil
		}
	}
	return "", common.ErrDevNotFound
}

func getDev(major, minor uint64) uint64 {
	return unix.Mkdev(uint32(major), uint32(minor))
}
