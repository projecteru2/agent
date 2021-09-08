package core

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

// Store use core to store meta
type Store struct {
	clientPool *ClientPool
	config     *types.Config
	cache      *cache.Cache
}

var coreStore *Store
var once sync.Once

// New new a Store
func New(ctx context.Context, config *types.Config) (*Store, error) {
	clientPool, err := NewCoreRPCClientPool(ctx, config)
	if err != nil {
		return nil, err
	}
	cache := cache.New(time.Duration(config.HealthCheck.CacheTTL)*time.Second, 24*time.Hour)
	return &Store{clientPool, config, cache}, nil
}

// GetClient returns a gRPC client
func (c *Store) GetClient() pb.CoreRPCClient {
	return c.clientPool.getClient()
}

// Init inits the core store only once
func Init(ctx context.Context, config *types.Config) {
	once.Do(func() {
		var err error
		coreStore, err = New(ctx, config)
		if err != nil {
			log.Errorf("[Init] failed to create core store, err: %v", err)
			return
		}
	})
}

// Get returns the core store instance
func Get() *Store {
	return coreStore
}
