package engine

import (
	"context"

	log "github.com/Sirupsen/logrus"
	types "github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
)

var eventHandler = status.NewEventHandler()

func (e *Engine) initMonitor() (<-chan eventtypes.Message, <-chan error) {
	eventHandler.Handle(common.STATUS_START, e.handleContainerStart)
	eventHandler.Handle(common.STATUS_DIE, e.handleContainerDie)
	eventHandler.Handle(common.STATUS_DESTROY, e.handleContainerDestory)

	ctx := context.Background()
	f := getFilter()
	f.Add("type", eventtypes.ContainerEventType)
	options := types.EventsOptions{Filters: f}
	eventChan, errChan := e.docker.Events(ctx, options)
	return eventChan, errChan
}

func (e *Engine) monitor(eventChan <-chan eventtypes.Message) {
	log.Info("[monitor] Status watch start")
	eventHandler.Watch(eventChan)
}

func (e *Engine) handleContainerStart(event eventtypes.Message) {
	log.Debugf("[handleContainerStart] container %s start", event.ID[:common.SHORTID])
	container, err := e.detectContainer(event.ID, event.Actor.Attributes)
	if err != nil {
		log.Errorf("[handleContainerStart] detect container failed %v", err)
		return
	}

	if container.Running {
		// 这货会自动退出
		e.attach(container)
	}

	if err := e.store.DeployContainer(container, e.node); err != nil {
		log.Errorf("[handleContainerStart] update deploy status failed %v", err)
	}
}

func (e *Engine) handleContainerDie(event eventtypes.Message) {
	log.Debugf("[handleContainerDie] container %s die", event.ID[:common.SHORTID])
	container, err := e.detectContainer(event.ID, event.Actor.Attributes)
	if err != nil {
		log.Errorf("[handleContainerDie] detect container failed %v", err)
	}

	if err := e.store.DeployContainer(container, e.node); err != nil {
		log.Errorf("[handleContainerDie] update deploy status failed %v", err)
	}
}

//Destroy by core, data removed by core
func (e *Engine) handleContainerDestory(event eventtypes.Message) {
	e.checker.Del(event.ID)
}
