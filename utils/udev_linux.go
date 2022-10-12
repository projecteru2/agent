//go:build linux
// +build linux

package utils

import (
	"io/ioutil" //nolint
	"os"
	"path"
	"syscall"

	"github.com/projecteru2/agent/common"
	"golang.org/x/sys/unix"
)

const (
	blkDevDir = "/dev/"
)

func GetDevicePath(major, minor uint64) (devPath string, err error) {
	files, err := ioutil.ReadDir(blkDevDir)
	if err != nil {
		return
	}
	dev := getDev(major, minor)
	for _, fi := range files {
		if (fi.Mode() & os.ModeDevice) == os.ModeDevice {
			stat, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				err = common.ErrSyscallFailed
				return
			}
			if stat.Rdev == dev {
				return path.Join(blkDevDir, fi.Name()), nil
			}
		}
	}
	return "", common.ErrDevNotFound
}

func getDev(major, minor uint64) (dev uint64) {
	return unix.Mkdev(uint32(major), uint32(minor))
}
