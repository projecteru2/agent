package manager

import (
	"context"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/source/docker"
	sourcemocks "github.com/projecteru2/agent/source/mocks"
	"github.com/projecteru2/agent/source/systemd"
	"github.com/projecteru2/agent/source/yavirt"
	"github.com/projecteru2/agent/store"
	corestore "github.com/projecteru2/agent/store/core"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

// Clients holds the shared per-process clients every manager runs on.
type Clients struct {
	Store  store.Store
	Source source.Source
}

// NewClients dials the store, looks this node up and dials the runtime the node hosts.
func NewClients(ctx context.Context, config *types.Config) (*Clients, error) {
	st, err := newStore(ctx, config)
	if err != nil {
		return nil, err
	}

	node, err := st.GetNode(ctx, config.HostName)
	if err != nil {
		return nil, err
	}
	nodeIP := utils.GetIP(node.Endpoint)
	if nodeIP == "" {
		nodeIP = common.LocalIP
	}

	src, err := newSource(ctx, config, nodeIP, st.GetIdentifier(ctx))
	if err != nil {
		return nil, err
	}
	return &Clients{Store: st, Source: src}, nil
}

func newStore(ctx context.Context, config *types.Config) (store.Store, error) {
	switch config.Store {
	case common.GRPCStore:
		return corestore.Get(ctx, config)
	case common.MocksStore:
		return storemocks.NewFakeStore(), nil
	default:
		return nil, common.ErrInvalidStoreType
	}
}

func newSource(ctx context.Context, config *types.Config, nodeIP, storeIdentifier string) (source.Source, error) {
	switch config.Runtime {
	case common.DockerRuntime:
		return docker.GetClient(ctx, config, nodeIP, storeIdentifier)
	case common.SystemdRuntime:
		return systemd.New(ctx, config)
	case common.YavirtRuntime:
		return yavirt.GetClient(ctx, config)
	case common.MocksRuntime:
		return sourcemocks.FromTemplate(), nil
	default:
		return nil, common.ErrInvalidRuntimeType
	}
}
