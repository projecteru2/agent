package status

import (
	"strings"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	coretypes "github.com/projecteru2/core/types"

	enginetypes "github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

// CalcuateCPUNum calculate how many cpu container used
func CalcuateCPUNum(container *types.Container, containerJSON enginetypes.ContainerJSON, hostCPUNum float64) *types.Container {
	cpuNum := hostCPUNum
	if containerJSON.HostConfig.CPUPeriod != 0 && containerJSON.HostConfig.CPUQuota != 0 {
		cpuNum = float64(containerJSON.HostConfig.CPUQuota) / float64(containerJSON.HostConfig.CPUPeriod)
	}
	container.CPUNum = cpuNum
	return container
}

func normalizeEnv(env []string) map[string]string {
	em := make(map[string]string)
	for _, e := range env {
		ps := strings.SplitN(e, "=", 2)
		if len(ps) == 2 {
			em[ps[0]] = ps[1]
		} else {
			em[ps[0]] = ""
		}
	}
	return em
}

// GenerateContainerMeta make meta obj
func GenerateContainerMeta(c enginetypes.ContainerJSON, meta *coretypes.LabelMeta, labels map[string]string) (*types.Container, error) {
	name, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return nil, err
	}

	container := &types.Container{
		StatusMeta:  coretypes.StatusMeta{ID: c.ID},
		Name:        name,
		EntryPoint:  entrypoint,
		Ident:       ident,
		Labels:      labels,
		Env:         normalizeEnv(c.Config.Env),
		HealthCheck: meta.HealthCheck,
		CPUQuota:    c.HostConfig.Resources.CPUQuota,
		CPUPeriod:   c.HostConfig.Resources.CPUPeriod,
		Memory:      utils.Max(c.HostConfig.Memory, c.HostConfig.MemoryReservation),
	}

	if !c.State.Running || c.State.Pid == 0 {
		container.Healthy = false
		container.Running = false
	} else {
		// 第一次上的容器可能没有设置health check
		// 那么我们认为这个容器一直是健康的, 并且不做检查
		// 需要告诉第一次上的时候这个容器是健康的, 还是不是
		container.Pid = c.State.Pid
		container.Running = c.State.Running
		container.Healthy = !(meta.HealthCheck != nil)
	}

	log.Debugf("[GenerateContainerMeta] Generate container meta %v %v", container.Name, container.EntryPoint)
	return container, nil
}
