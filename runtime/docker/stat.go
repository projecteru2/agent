package docker

import (
	"context"
	"strings"
	"time"

	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
	"github.com/shirou/gopsutil/net"
)

// CollectWorkloadMetrics .
func (d *Docker) CollectWorkloadMetrics(ctx context.Context, ID string) { //nolint
	// TODO
	// FIXME fuck internal pkg
	proc := "/proc"
	if utils.IsDockerized() {
		proc = "/hostProc"
	}
	logger := log.WithFunc("CollectWorkloadMetrics").WithField("ID", ID)

	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		logger.Error(ctx, err, "failed to detect container")
	}

	// init stats
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
	delta := float64(d.config.Metrics.Step)
	timeout := time.Duration(d.config.Metrics.Step) * time.Second
	tick := time.NewTicker(timeout)
	defer tick.Stop()
	hostname := strings.ReplaceAll(d.config.HostName, ".", "-")
	addr := ""
	if d.transfers.Len() > 0 {
		addr = d.transfers.Get(container.ID, 0)
	}

	period := float64(d.config.Metrics.Step)
	hostCPUCount := d.cpuCore * period

	mClient := NewMetricsClient(addr, hostname, container)
	defer logger.Infof(ctx, "container %s metric report stop", container.Name)
	logger.Infof(ctx, "container %s metric report start", container.Name)

	updateMetrics := func() {
		newContainer, err := d.detectWorkload(ctx, container.ID)
		if err != nil {
			logger.Error(ctx, err, "can not refresh container meta")
			return
		}
		containerCPUCount := newContainer.CPUNum * period
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		newContainerCPUStats, newSystemCPUStats, newContainerNetStats, err := getStats(timeoutCtx, newContainer.ID, newContainer.Pid, proc)
		if err != nil {
			logger.Error(ctx, err, "get stats failed")
			return
		}
		containerMemStats, err := getMemStats(timeoutCtx, newContainer.ID)
		if err != nil {
			logger.Error(ctx, err, "get mem stats failed")
			return
		}

		deltaContainerCPUUsage := newContainerCPUStats.Usage - containerCPUStats.Usage      // CPU Usage in seconds
		deltaContainerCPUSysUsage := newContainerCPUStats.System - containerCPUStats.System // Sys Usage in jiffies / tick
		deltaContainerCPUUserUsage := newContainerCPUStats.User - containerCPUStats.User    // User Usage in jiffies / tick

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

		cpuContainerUsage := deltaContainerCPUUsage / containerCPUCount // 实际消耗的 CPU 秒 / 允许消耗的 CPU 秒
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
		if newContainer.Memory > 0 {
			mClient.MemPercent(float64(containerMemStats.MemUsageInBytes) / float64(newContainer.Memory))
			mClient.MemRSSPercent(float64(containerMemStats.RSS) / float64(newContainer.Memory))
		}
		nics := map[string]net.IOCountersStat{}
		for _, nic := range containerNetStats {
			nics[nic.Name] = nic
		}
		for _, nic := range newContainerNetStats {
			if _, ok := nics[nic.Name]; !ok {
				continue
			}
			oldNICStats := nics[nic.Name]
			mClient.BytesSent(nic.Name, float64(nic.BytesSent-oldNICStats.BytesSent)/delta)
			mClient.BytesRecv(nic.Name, float64(nic.BytesRecv-oldNICStats.BytesRecv)/delta)
			mClient.PacketsSent(nic.Name, float64(nic.PacketsSent-oldNICStats.PacketsSent)/delta)
			mClient.PacketsRecv(nic.Name, float64(nic.PacketsRecv-oldNICStats.PacketsRecv)/delta)
			mClient.ErrIn(nic.Name, float64(nic.Errin-oldNICStats.Errin)/delta)
			mClient.ErrOut(nic.Name, float64(nic.Errout-oldNICStats.Errout)/delta)
			mClient.DropIn(nic.Name, float64(nic.Dropin-oldNICStats.Dropin)/delta)
			mClient.DropOut(nic.Name, float64(nic.Dropout-oldNICStats.Dropout)/delta)
		}
		logger.Debug(ctx, "start to get blkio stats for")
		newRawBlkioStats, err := d.getBlkioStats(ctx, container.ID)
		if err != nil {
			logger.Error(ctx, err, "get diskio stats failed")
			return
		}
		newBlkioStats, err := fromEngineBlkioStats(newRawBlkioStats)
		if err != nil {
			logger.Error(ctx, err, "get diskio stats failed")
			return
		}
		for _, entry := range newBlkioStats.IOServiceBytesReadRecursive {
			mClient.IOServiceBytesRead(entry.Dev, float64(entry.Value))
		}
		for _, entry := range newBlkioStats.IOServiceBytesWriteRecursive {
			mClient.IOServiceBytesWrite(entry.Dev, float64(entry.Value))
		}
		for _, entry := range newBlkioStats.IOServicedReadRecusive {
			mClient.IOServicedRead(entry.Dev, float64(entry.Value))
		}
		for _, entry := range newBlkioStats.IOServicedWriteRecusive {
			mClient.IOServicedWrite(entry.Dev, float64(entry.Value))
		}
		// update diff
		diffBlkioStats := getBlkIOMetricsDifference(blkioStats, newBlkioStats)
		for _, entry := range diffBlkioStats.IOServiceBytesReadRecursive {
			mClient.IOServiceBytesReadPerSecond(entry.Dev, float64(entry.Value)/delta)
		}
		for _, entry := range diffBlkioStats.IOServiceBytesWriteRecursive {
			mClient.IOServiceBytesWritePerSecond(entry.Dev, float64(entry.Value)/delta)
		}
		for _, entry := range diffBlkioStats.IOServicedReadRecusive {
			mClient.IOServicedReadPerSecond(entry.Dev, float64(entry.Value)/delta)
		}
		for _, entry := range diffBlkioStats.IOServicedWriteRecusive {
			mClient.IOServicedWritePerSecond(entry.Dev, float64(entry.Value)/delta)
		}
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
			mClient.Unregister()
			return
		}
	}
}
