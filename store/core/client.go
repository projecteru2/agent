package corestore

import (
	"fmt"
	"time"

	"context"

	"github.com/patrickmn/go-cache"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/client"
)

// CoreStore use core to store meta
type CoreStore struct {
	client *client.Client
	config *types.Config
	cache  *cache.Cache
}

// NewClient new a client
func NewClient(ctx context.Context, config *types.Config) (*CoreStore, error) {
	if config.Core == "" {
		return nil, fmt.Errorf("Core addr not set")
	}
	coreClient, err := client.NewClient(ctx, config.Core, config.Auth)
	if err != nil {
		return nil, err
	}
	cache := cache.New(time.Duration(config.HealthCheck.CacheTTL)*time.Second, 24*time.Hour)
	return &CoreStore{coreClient, config, cache}, nil
}
