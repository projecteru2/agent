package yavirt

import (
	"sync"

	"github.com/projecteru2/agent/types"

	"github.com/projecteru2/core/log"
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
			log.Error(nil, err, "[InitClient] failed to create yavirt client") //nolint
		}
	})
}

// GetClient .
func GetClient() *Yavirt {
	return yavirt
}
