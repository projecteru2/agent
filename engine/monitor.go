package engine

import (
	"context"

	types "github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
	coreutils "github.com/projecteru2/core/utils"
)

var eventHandler = status.NewEventHandler()

func (e *Engine) initMonitor() (<-chan eventtypes.Message, <-chan error) {
	eventHandler.Handle(common.StatusStart, e.handleContainerStart)
	eventHandler.Handle(common.StatusDie, e.handleContainerDie)

	ctx := context.Background()
	f := e.getFilter(map[string]string{"type": eventtypes.ContainerEventType})
	options := types.EventsOptions{Filters: f}
	eventChan, errChan := e.docker.Events(ctx, options)
	return eventChan, errChan
}

func (e *Engine) monitor(eventChan <-chan eventtypes.Message) {
	log.Info("[monitor] Status watch start")
	eventHandler.Watch(eventChan)
}

func (e *Engine) handleContainerStart(event eventtypes.Message) {
	log.Debugf("[handleContainerStart] container %s start", coreutils.ShortID(event.ID))
	container, err := e.detectContainer(event.ID)
	if err != nil {
		log.Errorf("[handleContainerStart] detect container failed %v", err)
		return
	}

	if container.Running {
		// 这货会自动退出
		e.attach(container)
	}

	// 发现需要 health check 立刻执行
	if container.Healthy {
		if err := e.store.SetContainerStatus(context.Background(), container, e.node); err != nil {
			log.Errorf("[handleContainerStart] update deploy status failed %v", err)
		}
	} else {
		go e.checkOneContainer(container)
	}
}

func (e *Engine) handleContainerDie(event eventtypes.Message) {
	log.Debugf("[handleContainerDie] container %s die", coreutils.ShortID(event.ID))
	container, err := e.detectContainer(event.ID)
	if err != nil {
		log.Errorf("[handleContainerDie] detect container failed %v", err)
	} else if err := e.store.SetContainerStatus(context.Background(), container, e.node); err != nil {
		log.Errorf("[handleContainerDie] update deploy status failed %v", err)
	}
}
