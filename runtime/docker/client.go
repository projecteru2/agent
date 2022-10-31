package docker

import (
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
		client, err = New(config, nodeIP)
		if err != nil {
			log.Error(nil, err, "[GetDockerClient] failed to make docker client") //nolint
		}
	})
}

// GetClient .
func GetClient() *Docker {
	return client
}
