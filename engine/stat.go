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
	t, c, _, n, err := getStats(s)
	if err != nil {
		log.Errorf("get stats failed %s", err)
		return
	}

	tick := time.NewTicker(time.Duration(e.config.Metrics.Step) * time.Second)
	defer tick.Stop()
	statsd := metric.NewStatsdClient(e.transfers.Get(container.ID, 0))
	var tagString string
	if len(container.Extend) > 0 {
		tag := []string{}
		for _, v := range container.Extend {
			tag = append(tag, fmt.Sprintf("%v", v))
		}
		tagString = fmt.Sprintf("%s.%s.%s", e.config.HostName, strings.Join(tag, "."), container.ID[:7])
	} else {
		tagString = fmt.Sprintf("%s.%s", e.config.HostName, container.ID[:7])
	}

	endpoint := fmt.Sprintf("%s.%s.%s", container.Name, container.Version, container.EntryPoint)
	defer log.Infof("container %s %s metric report stop", container.Name, container.ID[:7])
	log.Infof("container %s %s metric report start", container.Name, container.ID[:7])

	for {
		select {
		case <-tick.C:
			go func() {
				t2, c2, m, n2, err := getStats(s)
				if err != nil {
					log.Errorf("stat %s container %s failed %s", container.Name, container.ID[:7], err)
					return
				}
				result := map[string]float64{}
				deltaT := float64(t2 - t)
				result["cpu_usage_rate"] = float64(c2.UsageInUserMode-c.UsageInUserMode) / deltaT
				result["cpu_system_rate"] = float64(c2.UsageInSystemMode-c.UsageInSystemMode) / deltaT
				result["mem_usage"] = float64(m.Usage)
				result["mem_max_usage"] = float64(m.MaxUsage)
				result["mem_rss"] = float64(m.Detail["rss"])
				for k, v := range n2 {
					result[k+".rate"] = float64(v-n[k]) / float64(e.config.Metrics.Step)
				}
				t, c, n = t2, c2, n2
				statsd.Send(result, endpoint, tagString)
			}()
		case <-stop:
			close(stop)
			return
		}
	}
}

func getStats(s *metric.Stats) (uint64, *types.CPUStats, *types.MemoryStats, map[string]uint64, error) {
	total, err := s.GetTotalJiffies()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	cpu, err := s.GetCPUStats()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	memory, err := s.GetMemoryStats()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	network, err := s.GetNetworkStats()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	return total, cpu, memory, network, nil
}
