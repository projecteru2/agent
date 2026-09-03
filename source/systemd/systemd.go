package systemd

import (
	"context"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/projecteru2/core/log"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/source/meta"
	"github.com/projecteru2/agent/types"
)

const (
	stateActive   = "active"
	stateInactive = "inactive"
	stateFailed   = "failed"

	signalDepth = 100
	netnsFanout = 64
)

var _ source.Source = (*Systemd)(nil)

type Systemd struct {
	conn     *dbus.Conn
	dir      *meta.Dir
	reporter *source.Reporter
}

func New(ctx context.Context, config *types.Config) (*Systemd, error) {
	logger := log.WithFunc("systemd.New")
	logger.Infof(ctx, "systemd source starting, watching %s", config.MetaDir)

	dir, err := meta.NewDir(config.MetaDir, meta.KindProcess)
	if err != nil {
		logger.Errorf(ctx, err, "failed to create the meta dir %s", config.MetaDir)
		return nil, err
	}

	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to connect to the system bus")
		return nil, err
	}
	return &Systemd{conn: conn, dir: dir, reporter: source.NewReporter()}, nil
}

func (s *Systemd) List(ctx context.Context) ([]*source.Workload, error) {
	logger := log.WithFunc("systemd.List")

	files, err := s.dir.List(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to read the meta dir")
		return nil, err
	}

	running, err := s.runningUnits(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to list units")
		return nil, err
	}

	workloads := make([]*source.Workload, len(files))
	var g errgroup.Group
	g.SetLimit(netnsFanout)
	for i, f := range files {
		active := running[unitOf(f.ID)]
		s.reporter.Note(f.ID, source.ActionOf(active))
		g.Go(func() error {
			workloads[i] = s.withNetns(ctx, f.Workload(active))
			return nil
		})
	}
	_ = g.Wait()
	return workloads, nil
}

func (s *Systemd) Get(ctx context.Context, ID string) (*source.Workload, error) {
	f, err := s.dir.Read(ID)
	if err != nil {
		return nil, err
	}

	state, err := s.conn.GetUnitPropertyContext(ctx, unitOf(ID), "ActiveState")
	if err != nil {
		return nil, err
	}
	active, _ := state.Value.Value().(string)

	return s.withNetns(ctx, f.Workload(active == stateActive)), nil
}

func (s *Systemd) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	return source.PipeEvents(ctx, s.reporter,
		func(ctx context.Context) error { return s.dir.Watch(ctx, s.reporter) },
		s.watchUnits,
	)
}

func (s *Systemd) Alive(ctx context.Context) bool {
	if _, err := s.runningUnits(ctx); err != nil {
		log.WithFunc("systemd.Alive").Error(ctx, err, "connect to the system bus failed")
		return false
	}
	return true
}

func (s *Systemd) watchUnits(ctx context.Context) error {
	logger := log.WithFunc("systemd.watchUnits")
	// a previous Events left its subscription on this shared connection, still feeding dead channels
	_ = s.conn.Unsubscribe()
	if err := s.conn.Subscribe(); err != nil {
		return err
	}
	updates := make(chan *dbus.PropertiesUpdate, signalDepth)
	errs := make(chan error, signalDepth)
	s.conn.SetPropertiesSubscriber(updates, errs)
	if err := s.relist(ctx); err != nil {
		return err
	}

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
				s.reporter.Report(ID, action)
			}
		case err := <-errs:
			// a subscriber that fell behind missed transitions, it did not lose the bus
			logger.Warnf(ctx, "systemd subscription fell behind, relisting: %v", err)
			if err := s.relist(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// relist replays the state of every eru unit, so a transition the subscription missed is reconciled.
func (s *Systemd) relist(ctx context.Context) error {
	running, err := s.runningUnits(ctx)
	if err != nil {
		return err
	}
	for unit, active := range running {
		ID, ok := workloadIDFromUnit(unit)
		if !ok {
			continue
		}
		s.reporter.Report(ID, source.ActionOf(active))
	}
	return nil
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
