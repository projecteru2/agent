package status

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	eventtypes "github.com/docker/docker/api/types/events"
)

type EventHandler struct {
	sync.Mutex
	handlers map[string]func(eventtypes.Message)
}

func NewEventHandler() *EventHandler {
	return &EventHandler{handlers: make(map[string]func(eventtypes.Message))}
}

func (e *EventHandler) Handle(action string, h func(eventtypes.Message)) {
	e.Lock()
	e.handlers[action] = h
	e.Unlock()
}

func (e *EventHandler) Watch(c <-chan eventtypes.Message) {
	log.Infof("enter Watch")
	for ev := range c {
		log.Infof("cid %s action %s", ev.ID[:7], ev.Action)
		e.Lock()
		h, exists := e.handlers[ev.Action]
		e.Unlock()
		if !exists {
			continue
		}
		go h(ev)
	}
	log.Infof("exit Watch")
}
