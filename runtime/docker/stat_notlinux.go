//go:build !linux
// +build !linux

package docker

import (
	"context"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/docker"
	"github.com/shirou/gopsutil/v4/net"
)

func getStats(ctx context.Context, _ string, _ int, _ string) (*docker.CgroupCPUStat, cpu.TimesStat, []net.IOCountersStat, error) {
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

func getMemStats(context.Context, string) (*docker.CgroupMemStat, error) {
	return &docker.CgroupMemStat{}, nil
}
