package workload

import (
	"sync"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/cluster"
)

var filter map[string]string
var once sync.Once

func (m *Manager) getBaseFilter() map[string]string {
	once.Do(func() {
		filter = map[string]string{
			cluster.ERUMark: "1",
		}

		if m.config.CheckOnlyMine && utils.UseLabelAsFilter() {
			filter[common.ERUNodeName] = m.config.HostName
			if m.storeIdentifier != "" {
				filter[common.ERUCoreID] = m.storeIdentifier
			}
		}
	})

	return filter
}
