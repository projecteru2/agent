package workload

import (
	"fmt"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/cluster"
)

func (m *Manager) getFilter(extend map[string]string) []types.KV {
	var f []types.KV
	f = append(f, types.KV{Key: "label", Value: fmt.Sprintf("%s=1", cluster.ERUMark)})

	if m.config.CheckOnlyMine && utils.UseLabelAsFilter() {
		f = append(f, types.KV{Key: "label", Value: fmt.Sprintf("eru.nodename=%s", m.config.HostName)})
		if m.storeIdentifier != "" {
			f = append(f, types.KV{Key: "label", Value: fmt.Sprintf("eru.coreid=%s", m.storeIdentifier)})
		}
	}

	for k, v := range extend {
		f = append(f, types.KV{Key: k, Value: v})
	}
	return f
}
