package node

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/manager"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
)

type Manager struct {
	config *types.Config
	store  store.Store
	source source.Source

	statusMutex sync.Mutex
	exiting     bool
}

func NewManager(ctx context.Context, config *types.Config) (*Manager, error) {
	clients, err := manager.NewClients(ctx, config)
	if err != nil {
		log.WithFunc("node.NewManager").WithField("hostname", config.HostName).Error(ctx, err, "failed to create clients")
		return nil, err
	}
	return &Manager{config: config, store: clients.Store, source: clients.Source}, nil
}

func (m *Manager) Run(ctx context.Context) error {
	logger := log.WithFunc("node.Run")
	logger.Info(ctx, "start node status heartbeat")
	go m.heartbeat(ctx)

	<-ctx.Done()
	logger.Info(ctx, "exiting")
	return nil
}

func (m *Manager) Exit(ctx context.Context) error {
	logger := log.WithFunc("node.Exit").WithField("hostname", m.config.HostName)
	logger.Info(ctx, "remove node status")

	// a negative ttl removes the node status
	if err := m.setNodeStatus(ctx, -1); err != nil {
		logger.Error(ctx, err, "failed to remove node status")
		return err
	}
	return nil
}

func (m *Manager) setNodeStatus(ctx context.Context, ttl int64) error {
	m.statusMutex.Lock()
	defer m.statusMutex.Unlock()

	if m.exiting && ttl >= 0 {
		return nil
	}
	if ttl < 0 {
		m.exiting = true
	}
	return m.store.SetNodeStatus(ctx, ttl)
}
