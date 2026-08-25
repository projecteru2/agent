package docker

import (
	"context"
	"strings"

	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/utils"
)

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

func generateContainerMeta(ctx context.Context, c enginecontainer.InspectResponse, meta *coretypes.LabelMeta, labels map[string]string) (*Container, error) {
	name, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return nil, err
	}

	container := &Container{
		ID:          c.ID,
		Name:        name,
		EntryPoint:  entrypoint,
		Ident:       ident,
		Labels:      labels,
		Env:         normalizeEnv(c.Config.Env),
		HealthCheck: meta.HealthCheck,
		CPUQuota:    c.HostConfig.CPUQuota,
		CPUPeriod:   c.HostConfig.CPUPeriod,
		Memory:      max(c.HostConfig.Memory, c.HostConfig.MemoryReservation),
	}

	if !c.State.Running || c.State.Pid == 0 {
		container.Healthy = false
		container.Running = false
	} else {
		container.Pid = c.State.Pid
		container.Running = c.State.Running
		container.Healthy = meta.HealthCheck == nil
	}

	log.WithFunc("generateContainerMeta").Debugf(ctx, "Generate container meta %v %v", container.Name, container.EntryPoint)
	return container, nil
}

func calcuateCPUNum(container *Container, containerJSON enginecontainer.InspectResponse, hostCPUNum float64) *Container {
	cpuNum := hostCPUNum
	if containerJSON.HostConfig.CPUPeriod != 0 && containerJSON.HostConfig.CPUQuota != 0 {
		cpuNum = float64(containerJSON.HostConfig.CPUQuota) / float64(containerJSON.HostConfig.CPUPeriod)
	}
	container.CPUNum = cpuNum
	return container
}
