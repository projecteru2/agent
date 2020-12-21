package corestore

import (
	"context"
	"fmt"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/client"
)

// cache will cleanup every 15 seconds
const defaultCacheCleanupInterval = 15

// CoreStore use core to store meta
type CoreStore struct {
	client *client.Client
	config *types.Config
	cache  *Cache
}

// NewClient new a client
func NewClient(ctx context.Context, config *types.Config) (*CoreStore, error) {
	if config.Core == "" {
		return nil, fmt.Errorf("Core addr not set")
	}
	coreClient, err := client.NewClient(ctx, config.Core, config.Auth)
	return &CoreStore{coreClient, config, NewCache(defaultCacheCleanupInterval)}, err
}
