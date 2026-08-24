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

func InitClient(config *types.Config) {
	once.Do(func() {
		var err error
		yavirt, err = New(config)
		if err != nil {
			log.WithFunc("InitClient").Error(nil, err, "failed to create yavirt client") //nolint
		}
	})
}

func GetClient() *Yavirt {
	return yavirt
}
