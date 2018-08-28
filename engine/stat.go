package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/docker"
	"github.com/shirou/gopsutil/net"
	log "github.com/sirupsen/logrus"
)

func (e *Engine) stat(parentCtx context.Context, container *types.Container) {
	//TODO
	//FIXME fuck internal pkg
	proc := "/proc"
	if e.dockerized {
		proc = "/hostProc"
	}
	//init stats
	containerCPUStats, systemCPUStats, containerNetStats, err := getStats(parentCtx, container, proc)
	if err != nil {
		log.Errorf("[stat] get %s stats failed %v", container.ID[:common.SHORTID], err)
		return
	}

	delta := float64(e.config.Metrics.Step)
	tick := time.NewTicker(time.Duration(e.config.Metrics.Step) * time.Second)
	defer tick.Stop()
	hostname := strings.Replace(e.config.HostName, ".", "-", -1)
	version := strings.Replace(container.Version, ".", "-", -1) // redis 的版本号带了 '.' 导致监控数据格式不一致
	addr := ""
	if e.transfers.Len() > 0 {
		addr = e.transfers.Get(container.ID, 0)
	}
	mClient := NewMetricsClient(
		addr,
		container.ID,
		container.Name,
		container.EntryPoint,
		version,
		hostname,
	)
	defer log.Infof("[stat] container %s %s metric report stop", container.Name, container.ID[:common.SHORTID])
	log.Infof("[stat] container %s %s metric report start", container.Name, container.ID[:common.SHORTID])

	for {
		select {
		case <-tick.C:
			go func() {
				newContainrCPUStats, newSystemCPUStats, newContainerNetStats, err := getStats(parentCtx, container, proc)
				if err != nil {
					log.Errorf("[stat] get %s stats failed %v", container.ID[:common.SHORTID], err)
					return
				}
				containerMemStats, err := docker.CgroupMemDockerWithContext(parentCtx, container.ID)
				if err != nil {
					log.Errorf("[stat] get %s mem stats failed %v", container.ID[:common.SHORTID], err)
					return
				}
				cpuHostUsage := float64(newContainrCPUStats.Total()-containerCPUStats.Total()) / float64(newSystemCPUStats.Total()-systemCPUStats.Total())
				mClient.CPUHostUsage(cpuHostUsage)
				cpuHostUserUsage := float64(newContainrCPUStats.User-containerCPUStats.User) / float64(newSystemCPUStats.User-systemCPUStats.User)
				mClient.CPUHostUserUsage(cpuHostUserUsage)
				cpuHostSysUsage := float64(newContainrCPUStats.System-containerCPUStats.System) / float64(newSystemCPUStats.System-systemCPUStats.System)
				mClient.CPUHostSysUsage(cpuHostSysUsage)
				containerCPUPercent := container.CPUNum / e.cpuCore
				mClient.CPUContainerUsage(cpuHostUsage / containerCPUPercent)
				mClient.CPUContainerUserUsage(cpuHostUserUsage / containerCPUPercent)
				mClient.CPUContainerSysUsage(cpuHostSysUsage / containerCPUPercent)
				mClient.MemUsage(float64(containerMemStats.MemUsageInBytes))
				mClient.MemMaxUsage(float64(containerMemStats.MemMaxUsageInBytes))
				mClient.MemRss(float64(containerMemStats.RSS))
				if containerMemStats.MemLimitInBytes > 0 {
					mClient.MemPercent(float64(containerMemStats.MemUsageInBytes) / float64(containerMemStats.MemLimitInBytes))
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
				containerCPUStats, systemCPUStats, containerNetStats = newContainrCPUStats, newSystemCPUStats, newContainerNetStats
				mClient.Send()
			}()
		case <-parentCtx.Done():
			mClient.Unregister()
			return
		}
	}
}

func getStats(ctx context.Context, container *types.Container, proc string) (*cpu.TimesStat, cpu.TimesStat, []net.IOCountersStat, error) {
	//get container cpu stats
	containerCPUStats, err := docker.CgroupCPUDockerWithContext(ctx, container.ID)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	//get system cpu stats
	systemCPUsStats, err := cpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	systemCPUStats := systemCPUsStats[0]
	//get container nic stats
	netFilePath := fmt.Sprintf("%s/%d/net/dev", proc, container.Pid)
	containerNetStats, err := net.IOCountersByFileWithContext(ctx, true, netFilePath)
	if err != nil {
		return nil, cpu.TimesStat{}, []net.IOCountersStat{}, err
	}
	return containerCPUStats, systemCPUStats, containerNetStats, nil
}
