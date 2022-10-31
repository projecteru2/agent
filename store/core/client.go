package core

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/patrickmn/go-cache"
	"github.com/projecteru2/core/log"
)

// Store use core to store meta
type Store struct {
	clientPool *client.Pool
	config     *types.Config
	cache      *cache.Cache
}

var coreStore *Store
var once sync.Once

// New new a Store
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
	cache := cache.New(time.Duration(config.HealthCheck.CacheTTL)*time.Second, 24*time.Hour)
	return &Store{clientPool, config, cache}, nil
}

// GetClient returns a gRPC client
func (c *Store) GetClient() pb.CoreRPCClient {
	return c.clientPool.GetClient()
}

// Init inits the core store only once
func Init(ctx context.Context, config *types.Config) {
	once.Do(func() {
		var err error
		coreStore, err = New(ctx, config)
		if err != nil {
			log.Error(ctx, err, "[Init] failed to create core store")
			return
		}
	})
}

// Get returns the core store instance
func Get() *Store {
	return coreStore
}
