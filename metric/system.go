package metric

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

type Stats struct {
	cid string
	pid int

	interval  time.Duration
	bufReader *bufio.Reader

	cpuPath            string
	memoryUsagePath    string
	memoryMaxUsagePath string
	memoryDetailPath   string
	networkStatsPath   string
	statFilePath       string
	cpuQuotaRate       float64
}

func NewStats(container *types.Container) *Stats {
	s := &Stats{
		cid:       container.ID,
		pid:       container.Pid,
		bufReader: bufio.NewReaderSize(nil, 128),
	}
	procDir, statFile := "/proc", "/proc/stat"
	if os.Getenv("IN_DOCKER") != "" {
		procDir, statFile = "/hostProc", "/hostProc/stat"
	}
	s.statFilePath = statFile
	s.networkStatsPath = fmt.Sprintf("%s/%d/net/dev", procDir, s.pid)
	s.cpuPath = fmt.Sprintf(common.CGROUP_BASE_PATH, "cpu,cpuacct", container.ID, "cpuacct.stat")
	s.memoryUsagePath = fmt.Sprintf(common.CGROUP_BASE_PATH, "memory", container.ID, "memory.usage_in_bytes")
	s.memoryMaxUsagePath = fmt.Sprintf(common.CGROUP_BASE_PATH, "memory", container.ID, "memory.max_usage_in_bytes")
	s.memoryDetailPath = fmt.Sprintf(common.CGROUP_BASE_PATH, "memory", container.ID, "memory.stat")

	return s
}

func (s *Stats) GetTotalJiffies() (uint64, uint64, error) {
	var line string
	var tsReadingTotalJiffies = uint64(time.Now().Unix())

	f, err := os.Open(s.statFilePath)
	if err != nil {
		return 0, 0, err
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
				return 0, 0, fmt.Errorf("invalid number of cpu fields")
			}
			var totalJiffies uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				totalJiffies += v
			}
			return totalJiffies, tsReadingTotalJiffies, nil
		}
	}
	return 0, tsReadingTotalJiffies, fmt.Errorf("invalid stat format. Error trying to parse statfile: [%v]", s.statFilePath)
}
