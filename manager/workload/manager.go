package workload

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/manager"
	"github.com/projecteru2/agent/runtime"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

type Manager struct {
	config        *types.Config
	store         store.Store
	runtimeClient runtime.Runtime

	nodeIP     string
	forwards   *utils.HashBackends
	baseFilter map[string]string

	checkWorkloadMutex sync.Mutex
	startingWorkloads  map[string]*utils.RetryTask

	logBroadcaster *logBroadcaster

	storeIdentifier string
}

func NewManager(ctx context.Context, config *types.Config) (*Manager, error) {
	clients, err := manager.NewClients(ctx, config)
	if err != nil {
		log.WithFunc("workload.NewManager").Errorf(ctx, err, "failed to create clients for node %s", config.HostName)
		return nil, err
	}

	m := &Manager{
		config:            config,
		store:             clients.Store,
		runtimeClient:     clients.Runtime,
		nodeIP:            clients.NodeIP,
		forwards:          utils.NewHashBackends(config.Log.Forwards),
		logBroadcaster:    newLogBroadcaster(),
		startingWorkloads: map[string]*utils.RetryTask{},
	}
	m.storeIdentifier = m.store.GetIdentifier(ctx)
	m.baseFilter = newBaseFilter(config, m.storeIdentifier)
	return m, nil
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
