package workload

import (
	"github.com/projecteru2/core/cluster"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

func newBaseFilter(config *types.Config, storeIdentifier string) map[string]string {
	filter := map[string]string{cluster.ERUMark: "1"}
	if config.CheckOnlyMine && utils.UseLabelAsFilter() {
		filter[common.ERUNodeName] = config.HostName
		if storeIdentifier != "" {
			filter[common.ERUCoreID] = storeIdentifier
		}
	}
	return filter
}
