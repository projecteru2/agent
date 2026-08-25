package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
)

const (
	unitPrefix  = "eru-"
	unitSuffix  = ".service"
	unitPattern = unitPrefix + "*" + unitSuffix

	stateActive   = "active"
	stateInactive = "inactive"
	stateFailed   = "failed"

	eventType   = "process"
	signalDepth = 100
)

var _ source.Source = (*Systemd)(nil)

type Systemd struct {
	config *types.Config
	conn   *dbus.Conn
	dir    string
}

func New(ctx context.Context, config *types.Config) (*Systemd, error) {
	logger := log.WithFunc("systemd.New")
	logger.Infof(ctx, "systemd source starting, watching %s", config.MetaDir)

	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to connect to the system bus")
		return nil, err
	}
	return &Systemd{config: config, conn: conn, dir: config.MetaDir}, nil
}

func (s *Systemd) List(ctx context.Context) ([]*source.Workload, error) {
	logger := log.WithFunc("systemd.List")

	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		// the meta dir is on tmpfs, so a node that has not run a process workload yet has none
		logger.Debugf(ctx, "meta dir %s does not exist yet", s.dir)
		return nil, nil
	}
	if err != nil {
		logger.Error(ctx, err, "failed to read the meta dir")
		return nil, err
	}

	running, err := s.runningUnits(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to list units")
		return nil, err
	}

	workloads := make([]*source.Workload, 0, len(entries))
	for _, entry := range entries {
		ID, ok := workloadIDFromFile(entry.Name())
		if !ok {
			continue
		}
		m, err := readMeta(s.dir, ID)
		if err != nil {
			logger.Errorf(ctx, err, "failed to read the meta file of %s", ID)
			continue
		}
		workloads = append(workloads, m.workload(running[unitOf(ID)]))
	}
	return workloads, nil
}

func (s *Systemd) Get(ctx context.Context, ID string) (*source.Workload, error) {
	m, err := readMeta(s.dir, ID)
	if err != nil {
		return nil, err
	}

	props, err := s.conn.GetUnitPropertiesContext(ctx, unitOf(ID))
	if err != nil {
		return nil, err
	}
	state, _ := props["ActiveState"].(string)

	w := m.workload(state == stateActive)
	if pid, ok := props["MainPID"].(uint32); ok && needsNetns(w) {
		w.NetnsPID = int(pid)
	}
	return w, nil
}

func (s *Systemd) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	emit := func(ID, action string) {
		select {
		case eventChan <- &types.WorkloadEventMessage{ID: ID, Type: eventType, Action: action, TimeNano: time.Now().UnixNano()}:
		case <-ctx.Done():
		}
	}
	fail := func(err error) {
		cancel()
		select {
		case errChan <- err:
		default:
		}
	}

	go func() {
		defer cancel()
		defer close(eventChan)
		defer close(errChan)

		var wg sync.WaitGroup
		wg.Go(func() {
			if err := s.watchMetaDir(ctx, emit); err != nil {
				fail(err)
			}
		})
		wg.Go(func() {
			if err := s.watchUnits(ctx, emit); err != nil {
				fail(err)
			}
		})
		wg.Wait()
	}()

	return eventChan, errChan
}

func (s *Systemd) Alive(ctx context.Context) bool {
	if _, err := s.runningUnits(ctx); err != nil {
		log.WithFunc("systemd.Alive").Error(ctx, err, "connect to the system bus failed")
		return false
	}
	return true
}

// watchMetaDir turns a meta file appearing into discovery and its removal into removal.
func (s *Systemd) watchMetaDir(ctx context.Context, emit func(ID, action string)) error {
	watcher, err := newDirWatcher(s.dir)
	if err != nil {
		return err
	}

	err = watcher.run(ctx, func(name string, created bool) {
		ID, ok := workloadIDFromFile(name)
		if !ok {
			return
		}
		if created {
			emit(ID, common.StatusStart)
			return
		}
		emit(ID, common.StatusDie)
	})
	if errors.Is(err, os.ErrClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// watchUnits turns an ActiveState change of an eru unit into a start or a die.
func (s *Systemd) watchUnits(ctx context.Context, emit func(ID, action string)) error {
	if err := s.conn.Subscribe(); err != nil {
		return err
	}
	updates := make(chan *dbus.PropertiesUpdate, signalDepth)
	errs := make(chan error, signalDepth)
	s.conn.SetPropertiesSubscriber(updates, errs)

	for {
		select {
		case update := <-updates:
			ID, ok := workloadIDFromUnit(update.UnitName)
			if !ok {
				continue
			}
			changed, ok := update.Changed["ActiveState"]
			if !ok {
				continue
			}
			state, _ := changed.Value().(string)
			switch state {
			case stateActive:
				emit(ID, common.StatusStart)
			case stateInactive, stateFailed:
				emit(ID, common.StatusDie)
			}
		case err := <-errs:
			return err
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Systemd) runningUnits(ctx context.Context) (map[string]bool, error) {
	units, err := s.conn.ListUnitsByPatternsContext(ctx, nil, []string{unitPattern})
	if err != nil {
		return nil, err
	}
	running := make(map[string]bool, len(units))
	for _, unit := range units {
		running[unit.Name] = unit.ActiveState == stateActive
	}
	return running, nil
}

// needsNetns reports whether the workload has a network of its own, so the host counters are not its.
func needsNetns(w *source.Workload) bool {
	return len(w.Meta.Networks) > 0 && w.NetnsPID == 0 && w.HostIface == ""
}
