package workload

import (
	"bufio"
	"context"
	"io"
	"sync"

	"github.com/alphadose/haxmap"
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

	"github.com/projecteru2/core/log"
)

// Manager .
type Manager struct {
	config        *types.Config
	store         store.Store
	runtimeClient runtime.Runtime

	nodeIP   string
	forwards *utils.HashBackends

	checkWorkloadMutex *sync.Mutex
	startingWorkloads  *haxmap.Map[string, *utils.RetryTask]

	logBroadcaster *logBroadcaster

	// storeIdentifier indicates which eru this agent belongs to
	// it can be used to identify the corresponding core
	// and all containers that belong to this core
	storeIdentifier string
}

// NewManager returns a workload manager
func NewManager(ctx context.Context, config *types.Config) (*Manager, error) {
	m := &Manager{config: config}

	switch config.Store {
	case common.GRPCStore:
		corestore.Init(ctx, config)
		store := corestore.Get()
		if store == nil {
			return nil, common.ErrGetStoreFailed
		}
		m.store = store
	case common.MocksStore:
		m.store = storemocks.NewFakeStore()
	default:
		return nil, common.ErrInvalidStoreType
	}

	node, err := m.store.GetNode(ctx, config.HostName)
	if err != nil {
		log.Errorf(ctx, err, "[NewManager] failed to get node %s", config.HostName)
		return nil, err
	}

	nodeIP := utils.GetIP(node.Endpoint)
	if nodeIP == "" {
		nodeIP = common.LocalIP
	}

	switch config.Runtime {
	case common.DockerRuntime:
		docker.InitClient(config, nodeIP)
		m.runtimeClient = docker.GetClient()
		if m.runtimeClient == nil {
			return nil, common.ErrGetRuntimeFailed
		}
	case common.YavirtRuntime:
		yavirt.InitClient(config)
		m.runtimeClient = yavirt.GetClient()
		if m.runtimeClient == nil {
			return nil, common.ErrGetRuntimeFailed
		}
	case common.MocksRuntime:
		m.runtimeClient = runtimemocks.FromTemplate()
	default:
		return nil, common.ErrInvalidRuntimeType
	}

	m.logBroadcaster = newLogBroadcaster()
	m.forwards = utils.NewHashBackends(config.Log.Forwards)
	m.storeIdentifier = m.store.GetIdentifier(ctx)
	m.nodeIP = nodeIP
	m.checkWorkloadMutex = &sync.Mutex{}
	m.startingWorkloads = haxmap.New[string, *utils.RetryTask]()

	return m, nil
}

// Run will start agent
// blocks by ctx.Done()
// either call this in a separated goroutine, or used in main to block main goroutine
func (m *Manager) Run(ctx context.Context) error {
	// start log broadcaster
	_ = utils.Pool.Submit(func() { m.logBroadcaster.run(ctx) })

	// initWorkloadStatus container
	if err := m.initWorkloadStatus(ctx); err != nil {
		return err
	}

	// start status watcher
	_ = utils.Pool.Submit(func() { m.monitor(ctx) })

	// start health check
	_ = utils.Pool.Submit(func() { m.healthCheck(ctx) })

	// wait for signal
	<-ctx.Done()
	log.Info(ctx, "[WorkloadManager] exiting")
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
				log.Errorf(ctx, err, "[PullLog] %v failed to pull log", ID)
			}
			return
		}
	}
}
