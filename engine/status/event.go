package status

import (
	"context"
	"sync"

	eventtypes "github.com/docker/docker/api/types/events"
	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// EventHandler define event handler
type EventHandler struct {
	sync.Mutex
	handlers map[string]func(context.Context, eventtypes.Message)
}

// NewEventHandler new a event handler
func NewEventHandler() *EventHandler {
	return &EventHandler{handlers: make(map[string]func(context.Context, eventtypes.Message))}
}

// Handle hand a event
func (e *EventHandler) Handle(action string, h func(context.Context, eventtypes.Message)) {
	e.Lock()
	defer e.Unlock()
	e.handlers[action] = h
}

// Watch watch change
func (e *EventHandler) Watch(c <-chan eventtypes.Message) {
	for ev := range c {
		log.Infof("[Watch] Monitor: cid %s action %s", coreutils.ShortID(ev.ID), ev.Action)
		e.Lock()
		h := e.handlers[ev.Action]
		if h != nil {
			go h(context.TODO(), ev)
		}
		e.Unlock()
	}
}
