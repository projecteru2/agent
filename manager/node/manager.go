package node

import (
	"context"

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

// Manager manages node status
type Manager struct {
	config        *types.Config
	store         store.Store
	runtimeClient runtime.Runtime
}

// NewManager .
func NewManager(ctx context.Context, config *types.Config) (*Manager, error) {
	m := &Manager{config: config}

	switch config.Store {
	case common.GRPCStore:
		corestore.Init(ctx, config)
		if m.store = corestore.Get(); m.store == nil {
			return nil, common.ErrGetStoreFailed
		}
	case common.MocksStore:
		m.store = storemocks.NewFakeStore()
	default:
		return nil, common.ErrInvalidStoreType
	}

	node, err := m.store.GetNode(ctx, config.HostName)
	if err != nil {
		log.Errorf("[NewManager] failed to get node %s, err: %s", config.HostName, err)
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

	return m, nil
}

// Run runs a node manager
func (m *Manager) Run(ctx context.Context) error {
	log.Info("[NodeManager] start node status heartbeat")
	_ = utils.Pool.Submit(func() { m.heartbeat(ctx) })

	<-ctx.Done()
	log.Info("[NodeManager] exiting")
	return nil
}

// Exit .
func (m *Manager) Exit() error {
	log.Infof("[NodeManager] remove node status of %s", m.config.HostName)

	// ctx is now canceled. use a new context.
	var err error
	utils.WithTimeout(context.TODO(), m.config.GlobalConnectionTimeout, func(ctx context.Context) {
		// remove node status
		err = m.store.SetNodeStatus(ctx, -1)
	})
	if err != nil {
		log.Errorf("[NodeManager] failed to remove node status of %v, err: %s", m.config.HostName, err)
		return err
	}
	return nil
}
