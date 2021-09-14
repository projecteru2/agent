package docker

import (
	"context"
	"strings"
	"time"

	"github.com/projecteru2/agent/utils"

	"github.com/shirou/gopsutil/net"
	log "github.com/sirupsen/logrus"
)

// CollectWorkloadMetrics .
func (d *Docker) CollectWorkloadMetrics(ctx context.Context, ID string) {
	// TODO
	// FIXME fuck internal pkg
	proc := "/proc"
	if utils.IsDockerized() {
		proc = "/hostProc"
	}

	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		log.Errorf("[CollectWorkloadMetrics] failed to detect container, err: %v", err)
	}

	// init stats
	containerCPUStats, systemCPUStats, containerNetStats, err := getStats(ctx, container.ID, container.Pid, proc)
	if err != nil {
		log.Errorf("[stat] get %s stats failed %v", container.ID, err)
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
	defer log.Infof("[stat] container %s %s metric report stop", container.Name, container.ID)
	log.Infof("[stat] container %s %s metric report start", container.Name, container.ID)

	updateMetrics := func() {
		newContainer, err := d.detectWorkload(ctx, container.ID)
		if err != nil {
			log.Errorf("[stat] can not refresh container meta %s", container.ID)
			return
		}
		containerCPUCount := newContainer.CPUNum * period
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		newContainerCPUStats, newSystemCPUStats, newContainerNetStats, err := getStats(timeoutCtx, newContainer.ID, newContainer.Pid, proc)
		if err != nil {
			log.Errorf("[stat] get %s stats failed %v", newContainer.ID, err)
			return
		}
		containerMemStats, err := getMemStats(timeoutCtx, newContainer.ID)
		if err != nil {
			log.Errorf("[stat] get %s mem stats failed %v", newContainer.ID, err)
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
		containerCPUStats, systemCPUStats, containerNetStats = newContainerCPUStats, newSystemCPUStats, newContainerNetStats
		if err := mClient.Send(); err != nil {
			log.Errorf("[stat] Send metrics failed %v", err)
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
