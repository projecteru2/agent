package workload

import (
	"bufio"
	"context"
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

	"github.com/projecteru2/core/log"
)

type Manager struct {
	config        *types.Config
	store         store.Store
	runtimeClient runtime.Runtime

	nodeIP   string
	forwards *utils.HashBackends

	checkWorkloadMutex sync.Mutex
	startingWorkloads  map[string]*utils.RetryTask

	logBroadcaster *logBroadcaster

	// storeIdentifier names the eru cluster this agent belongs to
	storeIdentifier string
}

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
		log.WithFunc("NewManager").Errorf(ctx, err, "failed to get node %s", config.HostName)
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
	m.startingWorkloads = map[string]*utils.RetryTask{}

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
	log.WithFunc("Run").Info(ctx, "exiting")
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
			if err != io.EOF {
				log.WithFunc("PullLog").WithField("ID", ID).Error(ctx, err, "failed to pull log")
			}
			return
		}
	}
}
