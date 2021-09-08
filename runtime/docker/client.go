package docker

import (
	"sync"

	"github.com/projecteru2/agent/types"

	log "github.com/sirupsen/logrus"
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
			log.Errorf("[GetDockerClient] failed to make docker client, err: %s", err)
		}
	})
}

// GetClient .
func GetClient() *Docker {
	return client
}
