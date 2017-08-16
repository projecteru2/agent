package engine

import (
	"context"
	"os"

	log "github.com/Sirupsen/logrus"
	types "github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	filtertypes "github.com/docker/docker/api/types/filters"

	"github.com/coreos/etcd/client"
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
		return
	}
	//清理掉 ERU 标志
	delete(event.Actor.Attributes, "ERU")

	//看是否有元数据，有则是 crash 后重启
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		// 看看是不是 etcd 的 KeyNotFound error, 我们只需要对这种做进一步处理
		if etcdErr, ok := err.(client.Error); !ok || etcdErr.Code != client.ErrorCodeKeyNotFound {
			log.Error(err)
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
		log.Error(err)
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
	_, err := e.store.GetContainer(event.ID)
	if err != nil {
		hostname := os.Getenv("HOSTNAME")
		log.Errorf("while geting container from etcd, %s occured err: %v", hostname, err)
	}
	if err := e.store.RemoveContainer(event.ID); err != nil {
		hostname := os.Getenv("HOSTNAME")
		log.Errorf("while removing container, %s occured err: %v", hostname, err)
	}
	log.Infof("Monitor: container %s data removed", event.ID[:7])
}
