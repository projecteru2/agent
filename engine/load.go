package engine

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) load() error {
	log.Info("Load containers")
	containers, err := e.ListContainers(true)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// 拿到元信息
		containerFromCore, err := e.store.GetContainer(container.ID)
		if err != nil {
			log.Errorf("Load container stats failed %s", err)
			continue
		}
		containerFromCore.Engine = e.docker
		// 生成基准 meta
		delete(container.Labels, "ERU")
		version := "UNKNOWN"
		if v, ok := container.Labels["version"]; ok {
			version = v
		}
		delete(container.Labels, "version")
		containerInAgent, err := status.GenerateContainerMeta(containerFromCore, version, container.Labels)
		if err != nil {
			log.Errorf("Generate meta failed %s", err)
			continue
		}

		containerInAgent, err = e.currentInfo(containerFromCore, containerInAgent)
		if err != nil {
			log.Errorf("update container info failed %s", err)
			continue
		}

		// 非 eru-agent in docker 就转发日志，防止日志循环输出
		if _, ok := container.Labels["agent"]; !ok || !e.dockerized {
			e.attach(containerInAgent)
		}
	}
	return nil
}

func (e *Engine) currentInfo(containerFromCore *coretypes.Container, containerInAgent *types.Container) (*types.Container, error) {
	containerInfo, err := containerFromCore.Inspect()
	if err != nil {
		return nil, err
	}
	if !containerInfo.State.Running {
		return nil, fmt.Errorf("container %s [%s] not running", containerFromCore.Name, containerFromCore.ShortID())
	}
	containerInAgent.Pid = containerInfo.State.Pid
	containerInAgent.CPUQuota = containerInfo.HostConfig.Resources.CPUQuota
	containerInAgent.CPUPeriod = containerInfo.HostConfig.Resources.CPUPeriod
	containerInAgent.CPUShares = containerInfo.HostConfig.Resources.CPUShares
	containerInAgent.Memory = containerInfo.HostConfig.Resources.Memory
	containerInAgent.Healthy = e.judgeContainerHealth(containerInfo)
	containerFromCore.Healthy = containerInAgent.Healthy

	return containerInAgent, e.store.UpdateContainer(containerFromCore)
}
