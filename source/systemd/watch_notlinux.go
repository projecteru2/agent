//go:build !linux

package systemd

import (
	"context"
	"errors"
)

var errNoInotify = errors.New("watching the meta dir needs inotify")

type dirWatcher struct{}

func newDirWatcher(string) (*dirWatcher, error) {
	return nil, errNoInotify
}

func (w *dirWatcher) run(context.Context, func(name string, created bool)) error {
	return errNoInotify
}
