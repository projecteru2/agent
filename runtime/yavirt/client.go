package yavirt

import (
	"sync"

	"github.com/projecteru2/agent/types"

	log "github.com/sirupsen/logrus"
)

var (
	once   sync.Once
	yavirt *Yavirt
)

// InitClient init yavirt client
func InitClient(config *types.Config) {
	once.Do(func() {
		var err error
		yavirt, err = New(config)
		if err != nil {
			log.Errorf("[InitClient] failed to create yavirt client, err: %v", err)
		}
	})
}

// GetClient .
func GetClient() *Yavirt {
	return yavirt
}
