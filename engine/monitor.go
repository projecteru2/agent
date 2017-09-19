package engine

import (
	"context"

	log "github.com/Sirupsen/logrus"
	etcd "github.com/coreos/etcd/client"
	types "github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	filtertypes "github.com/docker/docker/api/types/filters"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
)

var eventHandler = status.NewEventHandler()

func (e *Engine) initMonitor() (<-chan eventtypes.Message, <-chan error) {
	eventHandler.Handle(common.STATUS_START, e.handleContainerStart)
	eventHandler.Handle(common.STATUS_DIE, e.handleContainerDie)
	eventHandler.Handle(common.STATUS_DESTROY, e.handleContainerDestroy)

	ctx := context.Background()
	f := filtertypes.NewArgs()
	f.Add("type", eventtypes.ContainerEventType)
	options := types.EventsOptions{Filters: f}
	eventChan, errChan := e.docker.Events(ctx, options)
	return eventChan, errChan
}

func (e *Engine) monitor(eventChan <-chan eventtypes.Message) {
	log.Info("Status watch start")
	eventHandler.Watch(eventChan)
}

func (e *Engine) handleContainerStart(event eventtypes.Message) {
	log.Debugf("container %s start", event.ID[:7])
	if _, ok := event.Actor.Attributes["ERU"]; !ok {
		log.Debugf("container %s is not deployed by eru", event.ID)
		return
	}
	//清理掉 ERU 标志
	delete(event.Actor.Attributes, "ERU")

	//看是否有元数据，有则是 crash 后重启
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		// 看看是不是 etcd 的 KeyNotFound error, 我们只需要对这种做进一步处理
		if !etcd.IsKeyNotFound(err) {
			log.Errorf("Load container stats failed %v", err)
			return
		}

		// 找不到说明需要重新从 label 生成数据
		log.Debug(event.Actor.Attributes)
		cname := event.Actor.Attributes["name"]
		version := "UNKNOWN"
		if v, ok := event.Actor.Attributes["version"]; ok {
			version = v
		}
		delete(event.Actor.Attributes, "name")
		delete(event.Actor.Attributes, "version")

		container, err = status.GenerateContainerMeta(event.ID, cname, version, event.Actor.Attributes)
		if err != nil {
			log.Error(err)
			return
		}
	}

	if err := e.bind(container, true); err != nil {
		log.Error(err)
		return
	}

	stop := make(chan int)
	e.attach(container, stop)
	go e.stat(container, stop)
}

func (e *Engine) handleContainerDie(event eventtypes.Message) {
	log.Debugf("container %s die", event.ID[:7])
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		if !etcd.IsKeyNotFound(err) {
			log.Errorf("Load container stats failed %v", err)
		}
		return
	}
	container.Alive = false
	container.Healthy = false
	if err := e.store.UpdateContainer(container); err != nil {
		log.Error(err)
	}
	log.Infof("Monitor: container %s data updated", event.ID[:7])
}

func (e *Engine) handleContainerDestroy(event eventtypes.Message) {
	log.Debugf("container %s destroy", event.ID[:7])
	if _, err := e.store.GetContainer(event.ID); err != nil {
		if !etcd.IsKeyNotFound(err) {
			log.Errorf("Load container stats failed %v", err)
		}
		return
	}
	if err := e.store.RemoveContainer(event.ID); err != nil {
		log.Errorf("while removing container, %s occured err: %v", e.hostname, err)
	}
	log.Infof("Monitor: container %s data removed", event.ID[:7])
}
