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
	eventHandler.Handle(common.STATUS_START, status.HandleContainerStart)
	eventHandler.Handle(common.STATUS_DIE, status.HandleContainerDie)

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
