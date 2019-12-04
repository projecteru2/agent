package corestore

import (
	"fmt"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/client"
)

// CoreStore use core to store meta
type CoreStore struct {
	client *client.Client
	config *types.Config
}

// NewClient new a client
func NewClient(config *types.Config) (*CoreStore, error) {
	if config.Core == "" {
		return nil, fmt.Errorf("Core addr not set")
	}
	coreClient := client.NewClient(config.Core, config.Auth)
	return &CoreStore{coreClient, config}, nil
}
