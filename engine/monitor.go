package engine

import (
	log "github.com/Sirupsen/logrus"
	types "github.com/docker/engine-api/types"
	eventtypes "github.com/docker/engine-api/types/events"
	filtertypes "github.com/docker/engine-api/types/filters"
	"golang.org/x/net/context"

	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/engine/status"
)

var eventHandler = status.NewEventHandler()

func (e *Engine) monitor() {
	eventHandler.Handle(common.STATUS_START, e.handleContainerStart)
	eventHandler.Handle(common.STATUS_DIE, e.handleContainerDie)
	eventHandler.Handle(common.STATUS_DESTROY, e.handleContainerDestroy)

	var eventChan = make(chan eventtypes.Message)
	go eventHandler.Watch(eventChan)
	e.monitorContainerEvents(eventChan)
	close(eventChan)
}

func (e *Engine) monitorContainerEvents(c chan eventtypes.Message) {
	ctx := context.Background()
	f := filtertypes.NewArgs()
	f.Add("type", "container")
	options := types.EventsOptions{Filters: f}
	resBody, err := e.docker.Events(ctx, options)
	// Whether we successfully subscribed to events or not, we can now
	// unblock the main goroutine.
	if err != nil {
		e.errChan <- err
		return
	}
	log.Info("Status watch start")
	defer resBody.Close()

	status.DecodeEvents(resBody, func(event eventtypes.Message, err error) error {
		if err != nil {
			e.errChan <- err
			return nil
		}
		c <- event
		return nil
	})
}

func (e *Engine) handleContainerStart(event eventtypes.Message) {
	log.Info(event.ID)
}

func (e *Engine) handleContainerDie(event eventtypes.Message) {
	log.Debugf("container %s die", event.ID[:7])
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		log.Error(err)
		return
	}
	if container == nil {
		return
	}
	container.Alive = false
	if err := e.store.UpdateContainer(container); err != nil {
		log.Error(err)
	}
}

func (e *Engine) handleContainerDestroy(event eventtypes.Message) {
	log.Debugf("container %s destroy", event.ID[:7])
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		log.Error(err)
		return
	}
	if container == nil {
		return
	}
	if err := e.store.RemoveContainer(event.ID); err != nil {
		log.Error(err)
	}
	log.Debugf("container %s removed", event.ID[:7])
}
