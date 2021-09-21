package workload

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"

	log "github.com/sirupsen/logrus"
)

// EventHandler define event handler
type EventHandler struct {
	sync.Mutex
	handlers map[string]func(context.Context, *types.WorkloadEventMessage)
}

// NewEventHandler new a event handler
func NewEventHandler() *EventHandler {
	return &EventHandler{handlers: make(map[string]func(context.Context, *types.WorkloadEventMessage))}
}

// Handle hand a event
func (e *EventHandler) Handle(action string, h func(context.Context, *types.WorkloadEventMessage)) {
	e.Lock()
	defer e.Unlock()
	e.handlers[action] = h
}

// Watch watch change
func (e *EventHandler) Watch(ctx context.Context, c <-chan *types.WorkloadEventMessage) {
	for {
		select {
		case ev, ok := <-c:
			if !ok {
				log.Infof("[Watch] event chan closed")
				return
			}
			log.Infof("[Watch] Monitor: workload id %s action %s", ev.ID, ev.Action)
			e.Lock()
			h := e.handlers[ev.Action]
			if h != nil {
				go h(ctx, ev)
			}
			e.Unlock()
		case <-ctx.Done():
			log.Infof("[Watch] context canceled, stop watching")
			return
		}
	}
}
