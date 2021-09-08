//go:build linux
// +build linux

package docker

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/docker"
	"github.com/shirou/gopsutil/net"
)

func getStats(ctx context.Context, ID string, pid int, proc string) (*docker.CgroupCPUStat, cpu.TimesStat, []net.IOCountersStat, error) {
	// get container cpu stats
	containerCPUStatsWithoutUsage, err := docker.CgroupCPUDockerWithContext(ctx, ID)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	containerCPUStatsUsage, err := docker.CgroupCPUDockerUsageWithContext(ctx, ID)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	containerCPUStats := &docker.CgroupCPUStat{
		TimesStat: *containerCPUStatsWithoutUsage,
		Usage:     containerCPUStatsUsage,
	}
	// get system cpu stats
	systemCPUsStats, err := cpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	// 0 means all cpu
	systemCPUStats := systemCPUsStats[0]
	// get container nic stats
	netFilePath := fmt.Sprintf("%s/%d/net/dev", proc, pid)
	containerNetStats, err := net.IOCountersByFileWithContext(ctx, true, netFilePath)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	return containerCPUStats, systemCPUStats, containerNetStats, nil
}

func getMemStats(ctx context.Context, ID string) (*docker.CgroupMemStat, error) {
	return docker.CgroupMemDockerWithContext(ctx, ID)
}
