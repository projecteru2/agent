//go:build linux
// +build linux

package utils

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

//  container env may not have udev package installed so we cant take this approach
//  const (
// 	BlockDeviceType = 'b'
// 	CharDeviceType  = 'c'
// )
//
// func GetDeviceFromDevnum(devType uint8, major int, minor int) (*udev.Device, error) {
// 	u := udev.Udev{}
// 	dev := u.NewDeviceFromDevnum(devType, udev.MkDev(major, minor))
// 	if dev == nil {
// 		return nil, errors.New("device not found")
// 	}
// 	return dev, nil
// }
//
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
				err = errors.New("syscall fail, Not a syscall.Stat_t")
				return
			}
			if stat.Rdev == dev {
				return path.Join(blkDevDir, fi.Name()), nil
			}
		}
	}
	return "", errors.New("device not found")
}

func getDev(major, minor uint64) (dev uint64) {
	return unix.Mkdev(uint32(major), uint32(minor))
}
