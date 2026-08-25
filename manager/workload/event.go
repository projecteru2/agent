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
	mu       sync.RWMutex
	handlers map[string]EventHandlerFunc
}

func NewEventHandler() *EventHandler {
	return &EventHandler{handlers: map[string]EventHandlerFunc{}}
}

func (e *EventHandler) Handle(action string, h EventHandlerFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[action] = h
}

func (e *EventHandler) Watch(ctx context.Context, c <-chan *types.WorkloadEventMessage) {
	logger := log.WithFunc("workload.Watch")
	for {
		select {
		case ev, ok := <-c:
			if !ok {
				logger.Info(ctx, "event chan closed")
				return
			}
			logger.Infof(ctx, "workload %s action %s", ev.ID, ev.Action)
			e.mu.RLock()
			h := e.handlers[ev.Action]
			e.mu.RUnlock()
			if h != nil {
				go h(ctx, ev)
			}
		case <-ctx.Done():
			logger.Info(ctx, "context canceled, stop watching")
			return
		}
	}
}
