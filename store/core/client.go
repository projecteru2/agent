package core

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"

	"github.com/patrickmn/go-cache"
	"github.com/projecteru2/core/log"
)

var (
	coreStore *Store
	once      sync.Once
)

type Store struct {
	clientPool *client.Pool
	config     *types.Config
	cache      *cache.Cache
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
	cache := cache.New(time.Duration(config.HealthCheck.CacheTTL)*time.Second, 24*time.Hour)
	return &Store{clientPool, config, cache}, nil
}

func (c *Store) GetClient() pb.CoreRPCClient {
	return c.clientPool.GetClient()
}

func Init(ctx context.Context, config *types.Config) {
	once.Do(func() {
		var err error
		coreStore, err = New(ctx, config)
		if err != nil {
			log.WithFunc("core.client").Error(ctx, err, "failed to create core store")
			return
		}
	})
}

func Get() *Store {
	return coreStore
}
