package workload

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/manager"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

type collectTask struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type Manager struct {
	config    *types.Config
	store     store.Store
	source    source.Source
	collector *collector.Collector
	journal   *collector.Journal

	forwards *utils.HashBackends
	cas      *utils.GroupCAS

	checkWorkloadMutex sync.Mutex
	startingWorkloads  map[string]*utils.RetryTask

	collectMutex sync.Mutex
	collecting   map[string]*collectTask

	logMutex   sync.RWMutex
	logTargets map[string]*logTarget

	writerMutex sync.Mutex
	writers     map[string]*forwardWriter

	logBroadcaster *logBroadcaster
	events         *EventHandler
}

func NewManager(ctx context.Context, config *types.Config, clients *manager.Clients) *Manager {
	m := &Manager{
		config:            config,
		store:             clients.Store,
		source:            clients.Source,
		collector:         collector.New(ctx, config),
		journal:           collector.NewJournal(config.StateDir),
		forwards:          utils.NewHashBackends(config.Log.Forwards),
		cas:               utils.NewGroupCAS(),
		logBroadcaster:    newLogBroadcaster(),
		startingWorkloads: map[string]*utils.RetryTask{},
		collecting:        map[string]*collectTask{},
		logTargets:        map[string]*logTarget{},
		writers:           map[string]*forwardWriter{},
	}
	m.events = NewEventHandler(m.handleWorkloadStart, m.handleWorkloadDie)
	return m
}

func (m *Manager) Run(ctx context.Context) error {
	// watching before the initial load means an event raised during it is handled, not missed
	go m.monitor(ctx)

	if err := m.initWorkloadStatus(ctx); err != nil {
		return err
	}

	// the journal reader starts once the load registered every target, so no backlog line is dropped
	go m.forwardJournal(ctx)

	go m.healthCheck(ctx)

	<-ctx.Done()
	log.WithFunc("workload.Run").Info(ctx, "exiting")
	return nil
}

func (m *Manager) PullLog(ctx context.Context, app string, buf *bufio.ReadWriter) {
	ID, errChan, unsubscribe := m.logBroadcaster.subscribe(ctx, app, buf)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			if !errors.Is(err, io.EOF) {
				log.WithFunc("workload.PullLog").WithField("ID", ID).Error(ctx, err, "failed to pull log")
			}
			return
		}
	}
}

func (m *Manager) start(ctx context.Context, w *source.Workload) {
	m.startCollecting(ctx, w)
	m.startForwarding(ctx, w)
}

func (m *Manager) stop(ID string) {
	m.stopCollecting(ID)
	m.stopForwarding(ID)
}

func (m *Manager) startCollecting(ctx context.Context, w *source.Workload) {
	m.collectMutex.Lock()
	defer m.collectMutex.Unlock()

	if task, ok := m.collecting[w.ID]; ok {
		select {
		case <-task.done:
			task.cancel()
		default:
			return
		}
	}

	sampleCtx, cancel := context.WithCancel(ctx)
	task := &collectTask{cancel: cancel, done: make(chan struct{})}
	m.collecting[w.ID] = task
	go func() {
		defer close(task.done)
		m.collector.Collect(sampleCtx, w, func() *source.Workload { return m.refreshed(ctx, w.ID) })
	}()
}

func (m *Manager) refreshed(ctx context.Context, ID string) *source.Workload {
	refresher, ok := m.source.(source.Refresher)
	if !ok {
		return nil
	}
	w, err := refresher.Refresh(ID)
	if err != nil {
		log.WithFunc("workload.refreshed").WithField("ID", ID).Debugf(ctx, "cannot re-read the metadata: %v", err)
		return nil
	}
	return w
}

// stopCollecting waits for the sampler out: it unregisters the gauges another sampler would re-register.
func (m *Manager) stopCollecting(ID string) {
	m.collectMutex.Lock()
	defer m.collectMutex.Unlock()

	task, ok := m.collecting[ID]
	if !ok {
		return
	}
	delete(m.collecting, ID)
	task.cancel()
	<-task.done
}
