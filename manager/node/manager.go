package node

import (
	"context"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/manager"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

type Manager struct {
	config *types.Config
	store  store.Store
	source source.Source
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

	var err error
	utils.WithTimeout(ctx, m.config.GlobalConnectionTimeout, func(ctx context.Context) {
		// a negative ttl removes the node status
		err = m.store.SetNodeStatus(ctx, -1)
	})
	if err != nil {
		logger.Error(ctx, err, "failed to remove node status")
		return err
	}
	return nil
}
