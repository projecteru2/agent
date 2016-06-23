package metric

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"gitlab.ricebook.net/platform/agent/types"
)

func (s *Stats) getCPUStats(cid string) (*types.CPUStats, error) {
	var line string
	cpuStats := &types.CPUStats{}
	f, err := os.Open(s.cpuPath)
	if err != nil {
		return nil, err
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
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid number of cpu fields")
		}
		v, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to convert value %s to int: %s", parts[1], err)
		}
		switch parts[0] {
		case "user":
			cpuStats.UsageInUserMode = v
		case "system":
			cpuStats.UsageInSystemMode = v
		}
	}
	if err == io.EOF {
		return cpuStats, nil
	}
	return nil, fmt.Errorf("invalid stat format. Error trying to parse the cpuacct file")
}

func (s *Stats) getContainerMemory(cid string) (*types.MemoryStats, error) {
	var line string
	memoryStats := &types.MemoryStats{}
	memoryStats.Detail = map[string]uint64{}
	usage, err := convert(s.memoryUsagePath)
	if err != nil {
		return nil, err
	}
	memoryStats.Usage = usage

	maxUsage, err = convert(s.memoryMaxUsagePath)
	if err != nil {
		return nil, err
	}
	memoryStats.MaxUsage = maxUsage

	f, err := os.Open(s.memoryDetailPath)
	if err != nil {
		return nil, err
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
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid number of memory detail fields")
		}
		v, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to convert value %s to int: %s", parts[1], err)
		}
		memoryStats.Detail[parts[0]] = v
	}
	if err == io.EOF {
		return memoryStats, nil
	}
	return nil, fmt.Errorf("invalid stat format. Error trying to parse the memory.stat file")
}

func convert(path string) (uint64, error) {
	f, err := os.Open(s.memoryUsagePath)
	defer f.Close()
	if err != nil {
		return 0, err
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(strings.Trim(string(b), 10, 64))
	return v, err
}
