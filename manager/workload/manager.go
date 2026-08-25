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

type Manager struct {
	config    *types.Config
	store     store.Store
	source    source.Source
	collector *collector.Collector

	forwards *utils.HashBackends
	cas      *utils.GroupCAS

	checkWorkloadMutex sync.Mutex
	startingWorkloads  map[string]*utils.RetryTask

	collectMutex sync.Mutex
	collecting   map[string]context.CancelFunc

	logBroadcaster *logBroadcaster
}

func NewManager(ctx context.Context, config *types.Config) (*Manager, error) {
	clients, err := manager.NewClients(ctx, config)
	if err != nil {
		log.WithFunc("workload.NewManager").Errorf(ctx, err, "failed to create clients for node %s", config.HostName)
		return nil, err
	}

	return &Manager{
		config:            config,
		store:             clients.Store,
		source:            clients.Source,
		collector:         collector.New(ctx, config),
		forwards:          utils.NewHashBackends(config.Log.Forwards),
		cas:               utils.NewGroupCAS(),
		logBroadcaster:    newLogBroadcaster(),
		startingWorkloads: map[string]*utils.RetryTask{},
		collecting:        map[string]context.CancelFunc{},
	}, nil
}

func (m *Manager) Run(ctx context.Context) error {
	go m.logBroadcaster.run(ctx)

	if err := m.initWorkloadStatus(ctx); err != nil {
		return err
	}

	go m.monitor(ctx)

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

// start attaches the workload's output and samples it, both idempotent per workload.
func (m *Manager) start(ctx context.Context, w *source.Workload) {
	m.startCollecting(ctx, w)
	go m.attach(ctx, w)
}

func (m *Manager) startCollecting(ctx context.Context, w *source.Workload) {
	m.collectMutex.Lock()
	defer m.collectMutex.Unlock()

	if cancel, ok := m.collecting[w.ID]; ok {
		cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	m.collecting[w.ID] = cancel
	go m.collector.Collect(ctx, w)
}

func (m *Manager) stopCollecting(ID string) {
	m.collectMutex.Lock()
	defer m.collectMutex.Unlock()

	if cancel, ok := m.collecting[ID]; ok {
		cancel()
		delete(m.collecting, ID)
	}
}
