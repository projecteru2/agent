package metric

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gitlab.ricebook.net/platform/agent/common"
)

func (s *Stats) GetNetworkStats() (map[string]uint64, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/net/dev", s.pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var d uint64
	var n [8]uint64
	result := map[string]uint64{}
	for scanner.Scan() {
		var name string
		text := scanner.Text()
		if strings.Index(text, ":") < 1 {
			continue
		}
		ts := strings.Split(text, ":")
		fmt.Sscanf(ts[0], "%s", &name)
		if !strings.HasPrefix(name, common.VLAN_PREFIX) && name != common.DEFAULT_BR {
			continue
		}
		fmt.Sscanf(ts[1],
			"%d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
			&n[0], &n[1], &n[2], &n[3], &d, &d, &d, &d,
			&n[4], &n[5], &n[6], &n[7], &d, &d, &d, &d,
		)
		result[name+".inbytes"] = n[0]
		result[name+".inpackets"] = n[1]
		result[name+".inerrs"] = n[2]
		result[name+".indrop"] = n[3]
		result[name+".outbytes"] = n[4]
		result[name+".outpackets"] = n[5]
		result[name+".outerrs"] = n[6]
		result[name+".outdrop"] = n[7]
	}
	return result, nil
}
