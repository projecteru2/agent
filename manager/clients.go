package manager

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
)

// Clients holds the shared per-process clients every manager runs on.
type Clients struct {
	Store   store.Store
	Runtime runtime.Runtime
	NodeIP  string
}

// NewClients dials the store, looks this node up and dials the runtime daemon.
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

	rt, err := newRuntime(ctx, config, nodeIP)
	if err != nil {
		return nil, err
	}
	return &Clients{Store: st, Runtime: rt, NodeIP: nodeIP}, nil
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

func newRuntime(ctx context.Context, config *types.Config, nodeIP string) (runtime.Runtime, error) {
	switch config.Runtime {
	case common.DockerRuntime:
		return docker.GetClient(ctx, config, nodeIP)
	case common.YavirtRuntime:
		return yavirt.GetClient(ctx, config)
	case common.MocksRuntime:
		return runtimemocks.FromTemplate(), nil
	default:
		return nil, common.ErrInvalidRuntimeType
	}
}
