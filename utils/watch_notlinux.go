//go:build !linux

package utils

import (
	"context"
	"errors"
)

var errNoInotify = errors.New("watching a path needs inotify")

type DirWatcher struct{}

func NewDirWatcher(string) (*DirWatcher, error) {
	return nil, errNoInotify
}

func (d *DirWatcher) Run(context.Context, func(name string, created bool)) error {
	return errNoInotify
}
