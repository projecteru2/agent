package corestore

import (
	"context"
	"time"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/patrickmn/go-cache"
)

// CoreStore use core to store meta
type CoreStore struct {
	clientPool RPCClientPool
	config     *types.Config
	cache      *cache.Cache
}

// New new a CoreStore
func New(ctx context.Context, config *types.Config) (*CoreStore, error) {
	clientPool, err := NewCoreRPCClientPool(ctx, config)
	if err != nil {
		return nil, err
	}
	cache := cache.New(time.Duration(config.HealthCheck.CacheTTL)*time.Second, 24*time.Hour)
	return &CoreStore{clientPool, config, cache}, nil
}

// GetClient returns a gRPC client
func (c *CoreStore) GetClient() pb.CoreRPCClient {
	return c.clientPool.GetClient()
}
