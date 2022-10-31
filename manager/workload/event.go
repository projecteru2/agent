package workload

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
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
				log.Info(ctx, "[Watch] event chan closed")
				return
			}
			log.Infof(ctx, "[Watch] Monitor: workload id %s action %s", ev.ID, ev.Action)
			e.Lock()
			if h := e.handlers[ev.Action]; h != nil {
				_ = utils.Pool.Submit(func() { h(ctx, ev) })
			}
			e.Unlock()
		case <-ctx.Done():
			log.Info(ctx, "[Watch] context canceled, stop watching")
			return
		}
	}
}
