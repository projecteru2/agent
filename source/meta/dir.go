package meta

import (
	"context"
	"errors"
	"os"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/utils"
)

// Dir is the meta dir core writes over ssh, seen by the runtime one kind of workload belongs to.
type Dir struct {
	path string
	kind Kind
}

// NewDir creates the dir: it is on tmpfs, so it is empty after a reboot and inotify has nothing to watch.
func NewDir(path string, kind Kind) (*Dir, error) {
	if err := os.MkdirAll(path, 0o755); err != nil { //nolint:gosec // core writes this dir over ssh as well
		return nil, err
	}
	return &Dir{path: path, kind: kind}, nil
}

// List returns the meta file of every workload of this dir's kind, skipping the ones it cannot read.
func (d *Dir) List(ctx context.Context) ([]*File, error) {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return nil, err
	}

	files := make([]*File, 0, len(entries))
	for _, entry := range entries {
		ID, ok := IDFromFile(entry.Name())
		if !ok || !IsID(ID) {
			continue
		}
		if f := d.readMine(ctx, ID); f != nil {
			files = append(files, f)
		}
	}
	return files, nil
}

// Read returns the meta file of one workload of this dir's kind.
func (d *Dir) Read(ID string) (*File, error) {
	return read(d.path, ID, d.kind)
}

func (d *Dir) Readable() error {
	_, err := os.ReadDir(d.path)
	return err
}

// Watch reports a meta file appearing as a start and its removal as a die, until ctx is done.
func (d *Dir) Watch(ctx context.Context, reporter *source.Reporter) error {
	watcher, err := utils.NewDirWatcher(d.path)
	if err != nil {
		return err
	}

	err = watcher.Run(ctx, func(name string, created bool) {
		ID, ok := IDFromFile(name)
		if !ok || !IsID(ID) {
			return
		}
		if created {
			if d.readMine(ctx, ID) != nil {
				reporter.Report(ID, common.StatusStart)
			}
			return
		}
		reporter.Gone(ID)
	})
	if errors.Is(err, os.ErrClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (d *Dir) readMine(ctx context.Context, ID string) *File {
	f, err := d.Read(ID)
	switch {
	case errors.Is(err, errOtherKind):
		return nil
	case err != nil:
		log.WithFunc("meta.readMine").Warnf(ctx, "skipping the meta file of %s: %v", ID, err)
		return nil
	}
	return f
}
