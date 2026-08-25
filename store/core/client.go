package core

import (
	"context"
	"sync"

	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
)

var (
	once      sync.Once
	coreStore *Store
	storeErr  error
)

type Store struct {
	clientPool *client.Pool
	config     *types.Config
	cache      *statusCache
}

func New(ctx context.Context, config *types.Config) (*Store, error) {
	clientPoolConfig := &client.PoolConfig{
		EruAddrs:          config.Core,
		Auth:              config.Auth,
		ConnectionTimeout: config.GlobalConnectionTimeout,
	}
	clientPool, err := client.NewCoreRPCClientPool(ctx, clientPoolConfig)
	if err != nil {
		return nil, err
	}
	return &Store{clientPool, config, newStatusCache()}, nil
}

func (c *Store) GetClient() pb.CoreRPCClient {
	return c.clientPool.GetClient()
}

// Get returns the process-wide core store, creating it on first call.
func Get(ctx context.Context, config *types.Config) (*Store, error) {
	once.Do(func() { coreStore, storeErr = New(ctx, config) })
	return coreStore, storeErr
}
