package workload

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

// EventHandlerFunc reacts to one workload event.
type EventHandlerFunc func(context.Context, *types.WorkloadEventMessage)

type EventHandler struct {
	start EventHandlerFunc
	die   EventHandlerFunc
	queue *serialQueue
}

func NewEventHandler(start, die EventHandlerFunc) *EventHandler {
	return &EventHandler{start: start, die: die, queue: newSerialQueue()}
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
			// one workload's events are applied in order, so a die cannot land after the next start
			switch ev.Action {
			case common.StatusStart:
				e.queue.Go(ev.ID, func() { e.start(ctx, ev) })
			case common.StatusDie:
				e.queue.Go(ev.ID, func() { e.die(ctx, ev) })
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

	queued, draining := q.pending[key]
	q.pending[key] = append(queued, task)
	if !draining {
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
		q.pending[key] = queued[1:]
		q.mu.Unlock()

		queued[0]()
	}
}
