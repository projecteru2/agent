//go:build !linux
// +build !linux

package docker

import (
	"context"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/docker"
	"github.com/shirou/gopsutil/net"
)

func getStats(ctx context.Context, ID string, pid int, proc string) (*docker.CgroupCPUStat, cpu.TimesStat, []net.IOCountersStat, error) {
	containerCPUStats := &docker.CgroupCPUStat{
		TimesStat: cpu.TimesStat{},
		Usage:     0.0,
	}
	// get system cpu stats
	systemCPUsStats, err := cpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	// 0 means all cpu
	systemCPUStats := systemCPUsStats[0]
	return containerCPUStats, systemCPUStats, []net.IOCountersStat{}, nil
}

func getMemStats(ctx context.Context, ID string) (*docker.CgroupMemStat, error) {
	return &docker.CgroupMemStat{}, nil
}
