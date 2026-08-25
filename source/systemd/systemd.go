package systemd

import (
	"context"
	"errors"
	"os"
	"strings"
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

type emitFunc func(ID, action string)

var _ source.Source = (*Systemd)(nil)

type Systemd struct {
	config *types.Config
	conn   *dbus.Conn
	dir    string

	reportedMutex sync.Mutex
	reported      map[string]string
}

func New(ctx context.Context, config *types.Config) (*Systemd, error) {
	logger := log.WithFunc("systemd.New")
	logger.Infof(ctx, "systemd source starting, watching %s", config.MetaDir)

	// the meta dir is on tmpfs, so it is empty after a reboot and inotify has nothing to watch
	if err := os.MkdirAll(config.MetaDir, 0o755); err != nil { //nolint:gosec // core writes this dir over ssh as well
		logger.Errorf(ctx, err, "failed to create the meta dir %s", config.MetaDir)
		return nil, err
	}

	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to connect to the system bus")
		return nil, err
	}
	return &Systemd{config: config, conn: conn, dir: config.MetaDir, reported: map[string]string{}}, nil
}

func (s *Systemd) List(ctx context.Context) ([]*source.Workload, error) {
	logger := log.WithFunc("systemd.List")

	entries, err := os.ReadDir(s.dir)
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
			logger.Warnf(ctx, "skipping the meta file of %s: %v", ID, err)
			continue
		}
		active := running[unitOf(ID)]
		s.report(unitOf(ID), actionOf(active))
		workloads = append(workloads, s.withNetns(ctx, m.workload(active)))
	}
	return workloads, nil
}

func (s *Systemd) Get(ctx context.Context, ID string) (*source.Workload, error) {
	m, err := readMeta(s.dir, ID)
	if err != nil {
		return nil, err
	}

	state, err := s.conn.GetUnitPropertyContext(ctx, unitOf(ID), "ActiveState")
	if err != nil {
		return nil, err
	}
	active, _ := state.Value.Value().(string)

	return s.withNetns(ctx, m.workload(active == stateActive)), nil
}

func (s *Systemd) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	emit := emitFunc(func(ID, action string) {
		select {
		case eventChan <- &types.WorkloadEventMessage{ID: ID, Type: eventType, Action: action, TimeNano: time.Now().UnixNano()}:
		case <-ctx.Done():
		}
	})
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

func (s *Systemd) watchMetaDir(ctx context.Context, emit emitFunc) error {
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
			s.emitChange(emit, ID, common.StatusStart)
			return
		}
		s.emitChange(emit, ID, common.StatusDie)
		s.forget(unitOf(ID))
	})
	if errors.Is(err, os.ErrClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (s *Systemd) watchUnits(ctx context.Context, emit emitFunc) error {
	logger := log.WithFunc("systemd.watchUnits")
	// a previous Events left its subscription on this shared connection, still feeding dead channels
	_ = s.conn.Unsubscribe()
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
				// the subscription is node wide, so only a name under eru's prefix is worth a line
				if strings.HasPrefix(update.UnitName, unitPrefix) {
					logger.Debugf(ctx, "ignoring unit %s, it is not a workload", update.UnitName)
				}
				continue
			}
			changed, ok := update.Changed["ActiveState"]
			if !ok {
				continue
			}
			if action, ok := actionFor(changed.Value()); ok {
				s.emitChange(emit, ID, action)
			}
		case err := <-errs:
			// a subscriber that fell behind missed transitions, it did not lose the bus
			logger.Warnf(ctx, "systemd subscription fell behind, relisting: %v", err)
			if err := s.relist(ctx, emit); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// relist replays the state of every eru unit, so a transition the subscription missed is reconciled.
func (s *Systemd) relist(ctx context.Context, emit emitFunc) error {
	running, err := s.runningUnits(ctx)
	if err != nil {
		return err
	}
	for unit, active := range running {
		ID, ok := workloadIDFromUnit(unit)
		if !ok {
			continue
		}
		s.emitChange(emit, ID, actionOf(active))
	}
	return nil
}

func (s *Systemd) emitChange(emit emitFunc, ID, action string) {
	if s.report(unitOf(ID), action) {
		emit(ID, action)
	}
}

func (s *Systemd) report(unit, action string) bool {
	s.reportedMutex.Lock()
	defer s.reportedMutex.Unlock()

	if s.reported[unit] == action {
		return false
	}
	s.reported[unit] = action
	return true
}

func (s *Systemd) forget(unit string) {
	s.reportedMutex.Lock()
	defer s.reportedMutex.Unlock()

	delete(s.reported, unit)
}

func (s *Systemd) runningUnits(ctx context.Context) (map[string]bool, error) {
	// the bus matches a glob only, so eru-agent.service comes back too and is dropped here
	units, err := s.conn.ListUnitsByPatternsContext(ctx, nil, []string{unitPattern})
	if err != nil {
		return nil, err
	}
	running := make(map[string]bool, len(units))
	for _, unit := range units {
		if _, ok := workloadIDFromUnit(unit.Name); !ok {
			continue
		}
		running[unit.Name] = unit.ActiveState == stateActive
	}
	return running, nil
}

// withNetns reads the pid off the unit: the meta file is written before the process exists.
func (s *Systemd) withNetns(ctx context.Context, w *source.Workload) *source.Workload {
	if !needsNetns(w) {
		return w
	}
	// MainPID lives on the Service interface, not the Unit one
	prop, err := s.conn.GetServicePropertyContext(ctx, unitOf(w.ID), "MainPID")
	if err != nil {
		log.WithFunc("systemd.withNetns").WithField("ID", w.ID).Warnf(ctx, "no main pid, so no network counters: %v", err)
		return w
	}
	if pid, ok := prop.Value.Value().(uint32); ok {
		w.NetnsPID = int(pid)
	}
	return w
}

func actionOf(active bool) string {
	if active {
		return common.StatusStart
	}
	return common.StatusDie
}

func actionFor(state any) (string, bool) {
	switch state {
	case stateActive:
		return common.StatusStart, true
	case stateInactive, stateFailed:
		return common.StatusDie, true
	}
	return "", false
}

// needsNetns reports whether the workload has a running network of its own, so the host counters are not its.
func needsNetns(w *source.Workload) bool {
	return w.Running && len(w.Meta.Networks) > 0 && w.NetnsPID == 0 && w.HostIface == ""
}
