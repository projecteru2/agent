package workload

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/runtime"
	"github.com/projecteru2/agent/runtime/docker"
	runtimemocks "github.com/projecteru2/agent/runtime/mocks"
	"github.com/projecteru2/agent/runtime/yavirt"
	"github.com/projecteru2/agent/store"
	corestore "github.com/projecteru2/agent/store/core"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	log "github.com/sirupsen/logrus"
)

// Manager .
type Manager struct {
	config        *types.Config
	store         store.Store
	runtimeClient runtime.Runtime

	nodeIP   string
	forwards *utils.HashBackends

	checkWorkloadMutex *sync.Mutex
	startingWorkloads  sync.Map

	logBroadcaster *logBroadcaster

	// storeIdentifier indicates which eru this agent belongs to
	// it can be used to identify the corresponding core
	// and all containers that belong to this core
	storeIdentifier string
}

// NewManager returns a workload manager
func NewManager(ctx context.Context, config *types.Config) (*Manager, error) {
	manager := &Manager{}
	var err error

	manager.config = config

	switch config.Store {
	case common.GRPCStore:
		corestore.Init(ctx, config)
		manager.store = corestore.Get()
		if manager.store == nil {
			log.Errorf("[NewManager] failed to create core store client")
			return nil, err
		}
	case common.MocksStore:
		manager.store = storemocks.FromTemplate()
	default:
		log.Errorf("[NewManager] unknown store type %s", config.Store)
	}

	node, err := manager.store.GetNode(ctx, config.HostName)
	if err != nil {
		log.Errorf("[NewManager] failed to get node %s, err: %s", config.HostName, err)
		return nil, err
	}

	manager.nodeIP = utils.GetIP(node.Endpoint)
	if manager.nodeIP == "" {
		manager.nodeIP = common.LocalIP
	}

	manager.forwards = utils.NewHashBackends(config.Log.Forwards)
	manager.storeIdentifier = manager.store.GetIdentifier(ctx)

	switch config.Runtime {
	case common.DockerRuntime:
		docker.InitClient(config, manager.nodeIP)
		manager.runtimeClient = docker.GetClient()
		if manager.runtimeClient == nil {
			log.Errorf("[NewManager] failed to create runtime client")
			return nil, err
		}
	case common.YavirtRuntime:
		yavirt.InitClient(config)
		manager.runtimeClient = yavirt.GetClient()
		if manager.runtimeClient == nil {
			return nil, errors.New("failed to get runtime client")
		}
	case common.MocksRuntime:
		manager.runtimeClient = runtimemocks.FromTemplate()
	default:
		log.Errorf("[NewManager] unknown runtime type %s", config.Runtime)
		return nil, err
	}

	manager.logBroadcaster = newLogBroadcaster()

	manager.checkWorkloadMutex = &sync.Mutex{}
	manager.startingWorkloads = sync.Map{}

	return manager, nil
}

// Run will start agent
// blocks by ctx.Done()
// either call this in a separated goroutine, or used in main to block main goroutine
func (m *Manager) Run(ctx context.Context) error {
	// start log broadcaster
	go m.logBroadcaster.run(ctx)

	// initWorkloadStatus container
	if err := m.initWorkloadStatus(ctx); err != nil {
		return err
	}

	// start status watcher
	go m.monitor(ctx)

	// start health check
	go m.healthCheck(ctx)

	// wait for signal
	<-ctx.Done()
	log.Info("[WorkloadManager] exiting")
	return nil
}

// PullLog pull logs for specific app
func (m *Manager) PullLog(ctx context.Context, app string, buf *bufio.ReadWriter) {
	ID, errChan, unsubscribe := m.logBroadcaster.subscribe(ctx, app, buf)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			if err != io.EOF {
				log.Errorf("[PullLog] %v failed to pull log, err: %v", ID, err)
			}
			return
		}
	}
}
