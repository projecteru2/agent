package metric

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
)

func (s *Stats) getNetworkStats() (map[string]types.NetStats, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/net/dev", s.pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var d uint64
	result := map[string]types.NetStats{}
	for scanner.Scan() {
		var name string
		var n [8]uint64
		text := scanner.Text()
		if strings.Index(text, ":") < 1 {
			continue
		}
		ts := strings.Split(text, ":")
		fmt.Sscanf(ts[0], "%s", &name)
		if !strings.HasPrefix(name, common.VLAN_PREFIX) && name != common.DEFAULT_BR {
			continue
		}
		result[name] = types.NetStats{}
		fmt.Sscanf(ts[1],
			"%d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
			&result[name].Inbytes, &result[name].Inpackets,
			&result[name].Inerrs, &result[name].Indrop,
			&d, &d, &d, &d,
			&result[name].Outbytes, &result[name].Outpackets,
			&result[name].Outerrs, &result[name].Outdrop,
			&d, &d, &d, &d,
		)
	}
	return result, nil
}
