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
	queue    *serialQueue
}

func NewEventHandler() *EventHandler {
	return &EventHandler{
		handlers: map[string]EventHandlerFunc{},
		queue:    newSerialQueue(),
	}
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
				// one workload's events are applied in order, so a die cannot land after the next start
				e.queue.Go(ev.ID, func() { h(ctx, ev) })
			}
		case <-ctx.Done():
			logger.Info(ctx, "context canceled, stop watching")
			return
		}
	}
}

// serialQueue runs the work submitted for one key in submission order, one task at a time,
// without making the submitter wait for the key that is busy.
type serialQueue struct {
	mu      sync.Mutex
	pending map[string][]func()
}

func newSerialQueue() *serialQueue {
	return &serialQueue{pending: map[string][]func(){}}
}

func (q *serialQueue) Go(key string, task func()) {
	q.mu.Lock()
	defer q.mu.Unlock()

	running := len(q.pending[key]) > 0
	q.pending[key] = append(q.pending[key], task)
	if !running {
		go q.drain(key)
	}
}

func (q *serialQueue) drain(key string) {
	for {
		q.mu.Lock()
		queued := q.pending[key]
		if len(queued) == 0 {
			delete(q.pending, key)
			q.mu.Unlock()
			return
		}
		task := queued[0]
		q.mu.Unlock()

		task()

		q.mu.Lock()
		q.pending[key] = q.pending[key][1:]
		q.mu.Unlock()
	}
}
