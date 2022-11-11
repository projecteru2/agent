package docker

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"

	"github.com/projecteru2/core/log"
)

var (
	once   sync.Once
	client *Docker
)

// InitClient init docker client
func InitClient(config *types.Config, nodeIP string) {
	once.Do(func() {
		var err error
		ctx := context.TODO()
		if client, err = New(ctx, config, nodeIP); err != nil {
			log.WithFunc("InitClient").Error(nil, err, "failed to make docker client") //nolint
		}
	})
}

// GetClient .
func GetClient() *Docker {
	return client
}
