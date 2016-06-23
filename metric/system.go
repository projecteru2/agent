package metric

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
)

type Stats struct {
	cid string
	pid int

	interval  time.Duration
	bufReader *bufio.Reader

	cpuPath string

	memoryUsagePath    string
	memoryMaxUsagePath string
	memoryDetailPath   string
}

func NewStats(container *types.Container, interval time.Duration) *Stats {
	s := &Stats{
		cid:       container.ID,
		pid:       container.Pid,
		interval:  interval,
		bufReader: bufio.NewReaderSize(nil, 128),
	}
	s.cpuPath = fmt.Sprintf(common.CGROUP_BASE_PATH, "cpu,cpuacct", cid, "cpuacct.stat")
	s.memoryUsagePath = fmt.Sprintf(common.CGROUP_BASE_PATH, "memory", cid, "memory.usage_in_bytes")
	s.memoryMaxUsagePath = fmt.Sprintf(common.CGROUP_BASE_PATH, "memory", cid, "memory.max_usage_in_bytes")
	s.memoryDetailPath = fmt.Sprintf(common.CGROUP_BASE_PATH, "memory", cid, "memory.stat")
	return s
}

func (s *Stats) getTotalJiffies() (uint64, error) {
	var line string
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer func() {
		s.bufReader.Reset(nil)
		f.Close()
	}()
	s.bufReader.Reset(f)
	err = nil
	for err == nil {
		line, err = s.bufReader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}
			var totalJiffies uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				totalJiffies += v
			}
			return totalJiffies, nil
		}
	}
	return 0, fmt.Errorf("invalid stat format. Error trying to parse the '/proc/stat' file")
}
