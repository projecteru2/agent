package core

import (
	"context"

	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
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
	return &Store{clientPool: clientPool, config: config, cache: newStatusCache()}, nil
}

func (c *Store) GetClient() pb.CoreRPCClient {
	return c.clientPool.GetClient()
}

func call[T any](ctx context.Context, c *Store, do func(ctx context.Context) (T, error)) (T, error) {
	var resp T
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		resp, err = do(ctx)
	})
	return resp, err
}
