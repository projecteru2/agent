package engine

import (
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/metric"
	"gitlab.ricebook.net/platform/agent/types"
)

func (e *Engine) stat(container *types.Container, stop chan int) {
	s := metric.NewStats(container)
	cpuQuotaRate := float64(container.CPUQuota) / float64(container.CPUPeriod) / e.cpuCore
	log.Debugf("CPUQuota: %d, CPUPeriod: %d, cpuQuotaRate: %f", container.CPUQuota, container.CPUPeriod, cpuQuotaRate)
	totalJiffies1, tsReadingTotalJiffies1, cpuStats1, _, networkStats1, err := getStats(s)
	if err != nil {
		log.Errorf("get stats failed %s", err)
		return
	}

	tick := time.NewTicker(time.Duration(e.config.Metrics.Step) * time.Second)
	defer tick.Stop()
	statsd := metric.NewStatsdClient(e.transfers.Get(container.ID, 0))
	var tagString string
	host := strings.Replace(e.config.HostName, ".", "-", -1)

	tagString = fmt.Sprintf("%s.%s", host, container.ID[:7])

	version := strings.Replace(container.Version, ".", "-", -1) // redis 的版本号带了 '.' 导致监控数据格式不一致
	endpoint := fmt.Sprintf("%s.%s.%s", container.Name, version, container.EntryPoint)
	defer log.Infof("container %s %s metric report stop", container.Name, container.ID[:7])
	log.Infof("container %s %s metric report start", container.Name, container.ID[:7])

	for {
		select {
		case <-tick.C:
			go func() {
				totalJiffies2, tsReadingTotalJiffies2, cpuStats2, memoryStats, networkStats2, err := getStats(s)
				if err != nil {
					log.Errorf("stat %s container %s failed %s", container.Name, container.ID[:7], err)
					return
				}
				result := map[string]float64{}
				cpuUsageRateServer, cpuSystemRateServer, cpuUsageRateContainer, cpuSystemRateContainer := e.calCPUrate(cpuStats1, cpuStats2, totalJiffies1, totalJiffies2, tsReadingTotalJiffies1, tsReadingTotalJiffies2, cpuQuotaRate)
				result["cpu_usage_rate_container"] = cpuUsageRateContainer
				result["cpu_system_rate_container"] = cpuSystemRateContainer
				result["cpu_usage_rate_server"] = cpuUsageRateServer
				result["cpu_system_rate_server"] = cpuSystemRateServer
				result["mem_usage"] = float64(memoryStats.Usage)
				result["mem_max_usage"] = float64(memoryStats.MaxUsage)
				result["mem_rss"] = float64(memoryStats.Detail["rss"])
				result["maxmemory"] = 0.0

				log.Debugf("container.Memory: %d", container.Memory)
				if container.Memory > 0 {
					result["mem_usage_rate"] = result["mem_usage"] / float64(container.Memory)
				}
				for k, v := range networkStats2 {
					result[k+".rate"] = float64(v-networkStats1[k]) / float64(e.config.Metrics.Step)
				}
				totalJiffies1, cpuStats1, networkStats1 = totalJiffies2, cpuStats2, networkStats2
				statsd.Send(result, endpoint, tagString)
			}()
		case <-stop:
			close(stop)
			return
		}
	}
}

func (e *Engine) calCPUrate(preCPUStat, postCPUStat *types.CPUStats, preTotal, postTotal, preTS, postTS uint64, quotaRate float64) (float64, float64, float64, float64) {
	deltaTimePre := (preCPUStat.ReadingTS - preTS) * 100 // sysconf(_SC_CLK_TCK):100
	log.Debugf("deltaTimePre: %d, preCPUStatTS: %d, preTS: %d", deltaTimePre, preCPUStat.ReadingTS, preTS)
	deltaTimePost := (postCPUStat.ReadingTS - postTS) * 100
	log.Debugf("deltaTimePost: %d, postCPUstatTs: %d, postTS: %d", deltaTimePost, postCPUStat.ReadingTS, postTS)
	deltaTotal := float64(postTotal - preTotal)
	log.Debugf("deltaTotal: %f", deltaTotal)
	cpuUsageRateServer := float64(postCPUStat.UsageInUserMode-preCPUStat.UsageInUserMode) / deltaTotal
	cpuSystemRateServer := float64(postCPUStat.UsageInSystemMode-preCPUStat.UsageInSystemMode) / deltaTotal
	cpuUsageRateContainer := cpuUsageRateServer / quotaRate
	cpuSystemRateContainer := cpuSystemRateServer / quotaRate
	log.Debugf("cpuUsageRateContainer: %f, cpuSystemRateContainer: %f", cpuUsageRateContainer, cpuSystemRateContainer)
	log.Debugf("cpuUsageRateServer: %f, cpuSystemRateServer: %f", cpuUsageRateServer, cpuSystemRateServer)
	return cpuUsageRateServer, cpuSystemRateServer, cpuUsageRateContainer, cpuSystemRateContainer
}

func getStats(s *metric.Stats) (uint64, uint64, *types.CPUStats, *types.MemoryStats, map[string]uint64, error) {
	total, ts, err := s.GetTotalJiffies()
	if err != nil {
		return 0, 0, nil, nil, nil, err
	}
	cpu, err := s.GetCPUStats()
	if err != nil {
		return 0, 0, nil, nil, nil, err
	}
	memory, err := s.GetMemoryStats()
	if err != nil {
		return 0, 0, nil, nil, nil, err
	}
	network, err := s.GetNetworkStats()
	if err != nil {
		return 0, 0, nil, nil, nil, err
	}
	return total, ts, cpu, memory, network, nil
}
