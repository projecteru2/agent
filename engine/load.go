package engine

import (
	"context"
	"strings"

	log "github.com/Sirupsen/logrus"
	etcd "github.com/coreos/etcd/client"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
)

func (e *Engine) load() error {
	log.Info("Load containers")
	ctx := context.Background()
	options := enginetypes.ContainerListOptions{All: true}

	containers, err := e.docker.ContainerList(ctx, options)
	if err != nil {
		return err
	}

	eruContainers := map[string]interface{}{}
	for _, container := range containers {
		c, err := e.store.GetContainer(container.ID)
		if err != nil {
			// etcd 错误
			if !etcd.IsKeyNotFound(err) {
				log.Errorf("Load container stats failed %s", err)
				continue
			}
			// 非 ERU 容器
			if _, ok := container.Labels["ERU"]; !ok {
				continue
			}
			// 生成基准 meta
			delete(container.Labels, "ERU")
			version := "UNKNOWN"
			if v, ok := container.Labels["version"]; ok {
				version = v
			}
			delete(container.Labels, "version")
			c, err = status.GenerateContainerMeta(container.ID, container.Names[0], version, container.Labels)
			if err != nil {
				log.Errorf("Generate meta failed %s", err)
				continue
			}
		}
		eruContainers[container.ID] = new(interface{})
		status := getStatus(container.Status)
		if err := e.bind(c, status); err != nil {
			log.Errorf("bind container info failed %s", err)
			continue
		}
		if !status {
			log.Warnf("%s container %s down", c.Name, c.ID[:7])
			continue
		}

		stop := make(chan int)
		// 非 eru-agent in docker 就转发日志，防止日志循环输出
		if _, ok := container.Labels["agent"]; !ok || !e.dockerized {
			e.attach(c, stop)
		}
		go e.stat(c, stop)
	}
	go func() {
		nodeAllContainers, err := e.store.GetAllContainers()
		if err != nil {
			log.Errorf("GetAllContainers Error: [%v]", err)
			return
		}
		for _, id := range nodeAllContainers {
			if _, ok := eruContainers[id]; ok {
				continue
			}
			log.Warnf("Remove Unknown Container [%v] From Etcd.", id)
			err = e.store.RemoveContainer(id)
			if err != nil {
				log.Errorf("Remove Container Error: [%v]", err)
			}
		}
	}()
	return nil
}

func (e *Engine) bind(container *types.Container, alive bool) error {
	c, err := e.docker.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return err
	}
	container.Pid = c.State.Pid
	container.CPUQuota = c.HostConfig.Resources.CPUQuota
	container.CPUPeriod = c.HostConfig.Resources.CPUPeriod
	container.CPUShares = c.HostConfig.Resources.CPUShares
	container.Memory = c.HostConfig.Resources.Memory
	container.Alive = alive
	container.Healthy = false
	if container.Alive {
		container.Healthy = e.judgeContainerHealth(c)
	}

	return e.store.UpdateContainer(container)
}

func getStatus(s string) bool {
	switch {
	case strings.HasPrefix(s, "Up"):
		return true
	default:
		return false
	}
}
