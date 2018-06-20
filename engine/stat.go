package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	statsdlib "github.com/CMGS/statsd"
	log "github.com/sirupsen/logrus"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/docker"
	"github.com/shirou/gopsutil/net"
)

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
	statsd := NewStatsdClient(e.transfers.Get(container.ID, 0))
	host := strings.Replace(e.config.HostName, ".", "-", -1)
	tagString := fmt.Sprintf("%s.%s", host, container.ID[:common.SHORTID])
	version := strings.Replace(container.Version, ".", "-", -1) // redis 的版本号带了 '.' 导致监控数据格式不一致
	endpoint := fmt.Sprintf("%s.%s.%s", container.Name, version, container.EntryPoint)
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
				result := map[string]float64{}
				result["cpu_usage"] = float64(newContainrCPUStats.Total()-containerCPUStats.Total()) / float64(newSystemCPUStats.Total()-systemCPUStats.Total())
				result["cpu_user_usage"] = float64(newContainrCPUStats.User-containerCPUStats.User) / float64(newSystemCPUStats.User-systemCPUStats.User)
				result["cpu_sys_usage"] = float64(newContainrCPUStats.System-containerCPUStats.System) / float64(newSystemCPUStats.System-systemCPUStats.System)
				result["mem_usage"] = float64(containerMemStats.MemUsageInBytes)
				result["mem_max_usage"] = float64(containerMemStats.MemMaxUsageInBytes)
				result["mem_rss"] = float64(containerMemStats.RSS)
				if containerMemStats.MemLimitInBytes > 0 {
					result["mem_usage_percent"] = float64(containerMemStats.MemUsageInBytes) / float64(containerMemStats.MemLimitInBytes)
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
					result[nic.Name+".bytes.sent"] = float64(nic.BytesSent-oldNICStats.BytesSent) / delta
					result[nic.Name+".bytes.recv"] = float64(nic.BytesRecv-oldNICStats.BytesRecv) / delta
					result[nic.Name+".packets.sent"] = float64(nic.PacketsSent-oldNICStats.PacketsSent) / delta
					result[nic.Name+".packets.sent"] = float64(nic.PacketsRecv-oldNICStats.PacketsRecv) / delta
					result[nic.Name+".err.in"] = float64(nic.Errin-oldNICStats.Errin) / delta
					result[nic.Name+".err.out"] = float64(nic.Errout-oldNICStats.Errout) / delta
					result[nic.Name+".drop.in"] = float64(nic.Dropin-oldNICStats.Dropin) / delta
					result[nic.Name+".drop.out"] = float64(nic.Dropout-oldNICStats.Dropout) / delta
				}
				containerCPUStats, systemCPUStats, containerNetStats = newContainrCPUStats, newSystemCPUStats, newContainerNetStats
				statsd.Send(result, endpoint, tagString)
			}()
		case <-parentCtx.Done():
			return
		}
	}
}

func NewStatsdClient(addr string) *StatsDClient {
	return &StatsDClient{
		Addr: addr,
	}
}

type StatsDClient struct {
	Addr string
}

func (self *StatsDClient) Close() error {
	return nil
}

func (self *StatsDClient) Send(data map[string]float64, endpoint, tag string) error {
	remote, err := statsdlib.New(self.Addr)
	if err != nil {
		log.Errorf("[statsd] Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	for k, v := range data {
		key := fmt.Sprintf("eru.%s.%s.%s", endpoint, tag, k)
		remote.Gauge(key, v)
	}
	return nil
}
