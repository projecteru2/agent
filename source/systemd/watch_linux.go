//go:build linux

package systemd

import (
	"context"
	"iter"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	watchMask = unix.IN_CREATE | unix.IN_MOVED_TO | unix.IN_DELETE | unix.IN_MOVED_FROM

	watchBufferSize = 64 << 10
)

type dirWatcher struct {
	f *os.File
}

// newDirWatcher arms an inotify watch on dir, so nothing that happens after it returns is missed.
func newDirWatcher(dir string) (*dirWatcher, error) {
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	// a non-blocking fd behind os.File rides the runtime poller, so Close unblocks the read
	f := os.NewFile(uintptr(fd), dir)
	if _, err := unix.InotifyAddWatch(fd, dir, watchMask); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &dirWatcher{f: f}, nil
}

func (w *dirWatcher) run(ctx context.Context, notify func(name string, created bool)) error {
	defer func() { _ = w.f.Close() }()
	go func() {
		<-ctx.Done()
		_ = w.f.Close()
	}()

	buf := make([]byte, watchBufferSize)
	for {
		n, err := w.f.Read(buf)
		if err != nil {
			return err
		}
		for name, created := range inotifyEvents(buf[:n]) {
			notify(name, created)
		}
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
