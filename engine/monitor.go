package engine

import (
	"context"

	log "github.com/Sirupsen/logrus"
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

	ctx := context.Background()
	f := filtertypes.NewArgs()
	f.Add("type", eventtypes.ContainerEventType)
	f.Add("label", "ERU=1")
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
	//看是否有元数据，有则是 crash 后重启
	containerFromCore, err := e.store.GetContainer(event.ID)
	if err != nil {
		log.Errorf("Load container stats failed %v", err)
		return
	}

	// 找不到说明需要重新从 label 生成数据
	log.Debug(event.Actor.Attributes)
	// 生成基准 meta
	delete(event.Actor.Attributes, "ERU")
	version := "UNKNOWN"
	if v, ok := event.Actor.Attributes["version"]; ok {
		version = v
	}
	delete(event.Actor.Attributes, "version")
	delete(event.Actor.Attributes, "name")

	containerInAgent, err := status.GenerateContainerMeta(containerFromCore, version, event.Actor.Attributes)
	if err != nil {
		log.Errorf("Generate meta failed %s", err)
		return
	}

	containerInAgent, err = e.currentInfo(containerFromCore, containerInAgent)
	if err != nil {
		log.Errorf("update container info failed %s", err)
		return
	}

	e.attach(containerInAgent)
}

func (e *Engine) handleContainerDie(event eventtypes.Message) {
	log.Debugf("container %s die", event.ID[:7])
	containerFromCore, err := e.store.GetContainer(event.ID)
	if err != nil {
		log.Errorf("Load container stats failed %v", err)
		return
	}

	containerFromCore.Healthy = false
	if err := e.store.UpdateContainer(containerFromCore); err != nil {
		log.Error(err)
	}
	log.Infof("Monitor: container %s data updated", event.ID[:7])
}

//Destroy by core, data removed by core
