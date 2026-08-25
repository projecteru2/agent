package cocoon

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/source/meta"
	"github.com/projecteru2/agent/types"
)

const (
	eventType = "vm"

	procsFile = "cgroup.procs"
)

var _ source.Source = (*Cocoon)(nil)

// Cocoon is the vm runtime: metadata from the meta dir, liveness from the cocoon daemon.
type Cocoon struct {
	config   *types.Config
	dir      *meta.Dir
	daemon   *daemon
	reporter *source.Reporter
}

func New(ctx context.Context, config *types.Config) (*Cocoon, error) {
	logger := log.WithFunc("cocoon.New")
	socket := config.Runtimes.Cocoon.Socket
	logger.Infof(ctx, "cocoon source starting, watching %s with the daemon on %s", config.MetaDir, socket)

	dir, err := meta.NewDir(config.MetaDir, meta.KindVM)
	if err != nil {
		logger.Errorf(ctx, err, "failed to create the meta dir %s", config.MetaDir)
		return nil, err
	}
	return &Cocoon{
		config:   config,
		dir:      dir,
		daemon:   newDaemon(socket, config.GlobalConnectionTimeout),
		reporter: source.NewReporter(),
	}, nil
}

func (c *Cocoon) List(ctx context.Context) ([]*source.Workload, error) {
	files, err := c.dir.List(ctx)
	if err != nil {
		log.WithFunc("cocoon.List").Error(ctx, err, "failed to read the meta dir")
		return nil, err
	}

	live := c.liveness(ctx)
	workloads := make([]*source.Workload, 0, len(files))
	for _, f := range files {
		w := workload(f, live)
		c.reporter.Note(w.ID, source.ActionOf(w.Running))
		workloads = append(workloads, w)
	}
	return workloads, nil
}

func (c *Cocoon) Get(ctx context.Context, ID string) (*source.Workload, error) {
	f, err := c.dir.Read(ID)
	if err != nil {
		return nil, err
	}
	return workload(f, c.liveness(ctx)), nil
}

func (c *Cocoon) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	emit := source.EmitFunc(func(ID, action string) {
		select {
		case eventChan <- &types.WorkloadEventMessage{ID: ID, Type: eventType, Action: action, TimeNano: time.Now().UnixNano()}:
		case <-ctx.Done():
		}
	})

	go func() {
		defer cancel()
		defer close(eventChan)
		defer close(errChan)

		var wg sync.WaitGroup
		wg.Go(func() { c.watchDaemon(ctx, emit) })
		wg.Go(func() {
			if err := c.dir.Watch(ctx, c.reporter, emit); err != nil {
				cancel()
				select {
				case errChan <- err:
				default:
				}
			}
		})
		wg.Wait()
	}()

	return eventChan, errChan
}

// Alive reports the daemon healthy; a socket that is not there means no daemon runs, which the scope check covers.
func (c *Cocoon) Alive(ctx context.Context) bool {
	err := c.daemon.health(ctx)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	log.WithFunc("cocoon.Alive").Error(ctx, err, "the cocoon daemon api is down")
	return false
}

// watchDaemon keeps reconnecting: the daemon is optional and restarts on its own, and its stream loses events.
func (c *Cocoon) watchDaemon(ctx context.Context, emit source.EmitFunc) {
	logger := log.WithFunc("cocoon.watchDaemon")
	for {
		err := c.daemon.events(ctx, func(ID string, running bool) {
			// the daemon supervises every vm on the node, so only a name eru created is worth an event
			if !meta.IsID(ID) {
				return
			}
			c.reporter.Report(emit, ID, source.ActionOf(running))
		})
		if ctx.Err() != nil {
			return
		}
		logger.Debugf(ctx, "no events from the cocoon daemon, will retry: %v", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(c.config.GlobalConnectionTimeout):
		}
	}
}

func (c *Cocoon) liveness(ctx context.Context) map[string]bool {
	live, err := c.daemon.vms(ctx)
	if err != nil {
		log.WithFunc("cocoon.liveness").Debugf(ctx, "no vm list from the cocoon daemon: %v", err)
	}
	return live
}

// workload falls back to the vm's own scope for a vm the daemon did not answer for.
func workload(f *meta.File, live map[string]bool) *source.Workload {
	running, ok := live[f.ID]
	if !ok {
		running = scopeAlive(f.Cgroup)
	}
	return f.Workload(running)
}

// scopeAlive reports whether the vm's cgroup scope still holds a process.
func scopeAlive(scope string) bool {
	procs, err := os.ReadFile(filepath.Join(scope, procsFile)) //nolint:gosec // the scope path comes from the meta file core wrote
	return err == nil && len(bytes.TrimSpace(procs)) > 0
}
