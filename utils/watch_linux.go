//go:build linux

package utils

import (
	"context"
	"iter"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	dirWatchMask = unix.IN_CREATE | unix.IN_MOVED_TO | unix.IN_DELETE | unix.IN_MOVED_FROM

	watchBufferSize = 64 << 10
)

// DirWatcher reports every entry that appears in or goes away from one directory.
type DirWatcher struct {
	w *inotify
}

// NewDirWatcher arms an inotify watch on dir, so nothing that happens after it returns is missed.
func NewDirWatcher(dir string) (*DirWatcher, error) {
	w, err := newInotify(dir, dirWatchMask)
	if err != nil {
		return nil, err
	}
	return &DirWatcher{w: w}, nil
}

// Run calls notify for every entry that changed, until ctx is done.
func (d *DirWatcher) Run(ctx context.Context, notify func(name string, created bool)) error {
	return d.w.run(ctx, func(buf []byte) {
		for name, created := range inotifyEvents(buf) {
			notify(name, created)
		}
	})
}

type inotify struct {
	f *os.File
}

func newInotify(dir string, mask uint32) (*inotify, error) {
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	// a non-blocking fd behind os.File rides the runtime poller, so Close unblocks the read
	f := os.NewFile(uintptr(fd), dir)
	if _, err := unix.InotifyAddWatch(fd, dir, mask); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &inotify{f: f}, nil
}

func (i *inotify) run(ctx context.Context, handle func(buf []byte)) error {
	defer func() { _ = i.f.Close() }()
	go func() {
		<-ctx.Done()
		_ = i.f.Close()
	}()

	buf := make([]byte, watchBufferSize)
	for {
		n, err := i.f.Read(buf)
		if err != nil {
			return err
		}
		handle(buf[:n])
	}
}

func inotifyEvents(buf []byte) iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		for len(buf) >= unix.SizeofInotifyEvent {
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[0])) //nolint:gosec // the kernel writes a packed InotifyEvent at the head of every record
			size := unix.SizeofInotifyEvent + int(event.Len)
			if size > len(buf) {
				return
			}
			name := strings.TrimRight(string(buf[unix.SizeofInotifyEvent:size]), "\x00")
			if name != "" && !yield(name, event.Mask&(unix.IN_CREATE|unix.IN_MOVED_TO) != 0) {
				return
			}
			buf = buf[size:]
		}
	}
}
