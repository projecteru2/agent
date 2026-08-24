package workload

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/types"
)

// EventHandlerFunc reacts to one workload event.
type EventHandlerFunc func(context.Context, *types.WorkloadEventMessage)

type EventHandler struct {
	sync.Mutex
	handlers map[string]EventHandlerFunc
}

func NewEventHandler() *EventHandler {
	return &EventHandler{handlers: map[string]EventHandlerFunc{}}
}

func (e *EventHandler) Handle(action string, h EventHandlerFunc) {
	e.Lock()
	defer e.Unlock()
	e.handlers[action] = h
}

func (e *EventHandler) Watch(ctx context.Context, c <-chan *types.WorkloadEventMessage) {
	logger := log.WithFunc("Watch")
	for {
		select {
		case ev, ok := <-c:
			if !ok {
				logger.Info(ctx, "event chan closed")
				return
			}
			logger.Infof(ctx, "monitor: workload id %s action %s", ev.ID, ev.Action)
			e.Lock()
			if h := e.handlers[ev.Action]; h != nil {
				go h(ctx, ev)
			}
			e.Unlock()
		case <-ctx.Done():
			logger.Info(ctx, "context canceled, stop watching")
			return
		}
	}
}
