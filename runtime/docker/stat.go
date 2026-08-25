package docker

import (
	"context"
	"strings"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/shirou/gopsutil/v4/net"

	"github.com/projecteru2/agent/utils"
)

func (d *Docker) CollectWorkloadMetrics(ctx context.Context, ID string) {
	proc := "/proc"
	if utils.IsDockerized() {
		proc = "/hostProc"
	}
	logger := log.WithFunc("docker.CollectWorkloadMetrics").WithField("ID", ID)

	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		logger.Error(ctx, err, "failed to detect container")
		return
	}

	containerCPUStats, systemCPUStats, containerNetStats, err := getStats(ctx, container.ID, container.Pid, proc)
	if err != nil {
		logger.Error(ctx, err, "get stats failed")
		return
	}
	rawBlkioStats, err := d.getBlkioStats(ctx, container.ID)
	if err != nil {
		logger.Error(ctx, err, "get diskio stats failed")
		return
	}
	blkioStats, err := fromEngineBlkioStats(rawBlkioStats)
	if err != nil {
		logger.Error(ctx, err, "get diskio stats failed")
		return
	}
	step := float64(d.config.Metrics.Step)
	timeout := time.Duration(d.config.Metrics.Step) * time.Second
	tick := time.NewTicker(timeout)
	defer tick.Stop()
	hostname := strings.ReplaceAll(d.config.HostName, ".", "-")
	addr := ""
	if d.transfers.Len() > 0 {
		addr = d.transfers.Get(container.ID, 0)
	}

	hostCPUCount := d.cpuCore * step

	mClient := NewMetricsClient(addr, hostname, container)
	defer logger.Infof(ctx, "container %s metric report stop", container.Name)
	logger.Infof(ctx, "container %s metric report start", container.Name)

	updateMetrics := func() {
		current := mClient.container.Load()
		containerCPUCount := current.CPUNum * step
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		newContainerCPUStats, newSystemCPUStats, newContainerNetStats, err := getStats(timeoutCtx, current.ID, current.Pid, proc)
		if err != nil {
			logger.Error(ctx, err, "get stats failed")
			return
		}
		containerMemStats, err := getMemStats(timeoutCtx, current.ID)
		if err != nil {
			logger.Error(ctx, err, "get mem stats failed")
			return
		}

		deltaContainerCPUUsage := newContainerCPUStats.Usage - containerCPUStats.Usage      // seconds
		deltaContainerCPUSysUsage := newContainerCPUStats.System - containerCPUStats.System // jiffies, not seconds
		deltaContainerCPUUserUsage := newContainerCPUStats.User - containerCPUStats.User

		deltaSystemCPUSysUsage := newSystemCPUStats.System - systemCPUStats.System
		deltaSystemCPUUserUsage := newSystemCPUStats.User - systemCPUStats.User

		cpuHostUsage := deltaContainerCPUUsage / hostCPUCount
		cpuHostSysUsage := 0.0
		if deltaSystemCPUSysUsage > 0 {
			cpuHostSysUsage = deltaContainerCPUSysUsage / deltaSystemCPUSysUsage
		}
		cpuHostUserUsage := 0.0
		if deltaSystemCPUUserUsage > 0 {
			cpuHostUserUsage = deltaContainerCPUUserUsage / deltaSystemCPUUserUsage
		}
		mClient.CPUHostUsage(cpuHostUsage)
		mClient.CPUHostSysUsage(cpuHostSysUsage)
		mClient.CPUHostUserUsage(cpuHostUserUsage)

		cpuContainerUsage := deltaContainerCPUUsage / containerCPUCount
		cpuContainerSysUsage := 0.0
		if deltaContainerCPUUsage > 0 {
			cpuContainerSysUsage = deltaContainerCPUSysUsage / deltaContainerCPUUsage
		}
		cpuContainerUserUsage := 0.0
		if deltaContainerCPUUsage > 0 {
			cpuContainerUserUsage = deltaContainerCPUUserUsage / deltaContainerCPUUsage
		}
		mClient.CPUContainerUsage(cpuContainerUsage)
		mClient.CPUContainerSysUsage(cpuContainerSysUsage)
		mClient.CPUContainerUserUsage(cpuContainerUserUsage)

		mClient.MemUsage(float64(containerMemStats.MemUsageInBytes))
		mClient.MemMaxUsage(float64(containerMemStats.MemMaxUsageInBytes))
		mClient.MemRss(float64(containerMemStats.RSS))
		if current.Memory > 0 {
			mClient.MemPercent(float64(containerMemStats.MemUsageInBytes) / float64(current.Memory))
			mClient.MemRSSPercent(float64(containerMemStats.RSS) / float64(current.Memory))
		}
		nics := make(map[string]net.IOCountersStat, len(containerNetStats))
		for _, nic := range containerNetStats {
			nics[nic.Name] = nic
		}
		for _, nic := range newContainerNetStats {
			old, ok := nics[nic.Name]
			if !ok {
				continue
			}
			mClient.BytesSent(nic.Name, float64(nic.BytesSent-old.BytesSent)/step)
			mClient.BytesRecv(nic.Name, float64(nic.BytesRecv-old.BytesRecv)/step)
			mClient.PacketsSent(nic.Name, float64(nic.PacketsSent-old.PacketsSent)/step)
			mClient.PacketsRecv(nic.Name, float64(nic.PacketsRecv-old.PacketsRecv)/step)
			mClient.ErrIn(nic.Name, float64(nic.Errin-old.Errin)/step)
			mClient.ErrOut(nic.Name, float64(nic.Errout-old.Errout)/step)
			mClient.DropIn(nic.Name, float64(nic.Dropin-old.Dropin)/step)
			mClient.DropOut(nic.Name, float64(nic.Dropout-old.Dropout)/step)
		}
		logger.Debug(ctx, "getting blkio stats")
		newRawBlkioStats, err := d.getBlkioStats(timeoutCtx, container.ID)
		if err != nil {
			logger.Error(ctx, err, "get diskio stats failed")
			return
		}
		newBlkioStats, err := fromEngineBlkioStats(newRawBlkioStats)
		if err != nil {
			logger.Error(ctx, err, "get diskio stats failed")
			return
		}
		publishBlkIO(newBlkioStats.IOServiceBytesReadRecursive, 1, mClient.IOServiceBytesRead)
		publishBlkIO(newBlkioStats.IOServiceBytesWriteRecursive, 1, mClient.IOServiceBytesWrite)
		publishBlkIO(newBlkioStats.IOServicedReadRecursive, 1, mClient.IOServicedRead)
		publishBlkIO(newBlkioStats.IOServicedWriteRecursive, 1, mClient.IOServicedWrite)

		diffBlkioStats := getBlkIOMetricsDifference(blkioStats, newBlkioStats)
		publishBlkIO(diffBlkioStats.IOServiceBytesReadRecursive, step, mClient.IOServiceBytesReadPerSecond)
		publishBlkIO(diffBlkioStats.IOServiceBytesWriteRecursive, step, mClient.IOServiceBytesWritePerSecond)
		publishBlkIO(diffBlkioStats.IOServicedReadRecursive, step, mClient.IOServicedReadPerSecond)
		publishBlkIO(diffBlkioStats.IOServicedWriteRecursive, step, mClient.IOServicedWritePerSecond)

		rawBlkioStats, blkioStats = newRawBlkioStats, newBlkioStats
		containerCPUStats, systemCPUStats, containerNetStats = newContainerCPUStats, newSystemCPUStats, newContainerNetStats
		if err := mClient.Send(ctx); err != nil {
			logger.Error(ctx, err, "send metrics failed")
		}
	}
	for {
		select {
		case <-tick.C:
			updateMetrics()
		case <-ctx.Done():
			removeMetricsClient(container.ID)
			return
		}
	}
}

func publishBlkIO(entries []*BlkIOEntry, div float64, set func(dev string, value float64)) {
	for _, entry := range entries {
		set(entry.Dev, float64(entry.Value)/div)
	}
}
