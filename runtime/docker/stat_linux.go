//go:build linux
// +build linux

package docker

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/docker"
	"github.com/shirou/gopsutil/v4/net"
)

func getStats(ctx context.Context, ID string, pid int, proc string) (*docker.CgroupCPUStat, cpu.TimesStat, []net.IOCountersStat, error) {
	containerCPUStats, err := docker.CgroupCPUDockerWithContext(ctx, ID)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	containerCPUStats.Usage, err = docker.CgroupCPUDockerUsageWithContext(ctx, ID)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	systemCPUsStats, err := cpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	// index 0 aggregates every cpu
	systemCPUStats := systemCPUsStats[0]
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
