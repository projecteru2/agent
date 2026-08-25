package source

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/agent/types"
)

type WatchFunc func(ctx context.Context) error

func PipeEvents(ctx context.Context, reporter *Reporter, eventType string, watchers ...WatchFunc) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	reporter.Attach(func(ID, action string) {
		select {
		case eventChan <- &types.WorkloadEventMessage{ID: ID, Type: eventType, Action: action, TimeNano: time.Now().UnixNano()}:
		case <-ctx.Done():
		}
	})
	fail := func(err error) {
		cancel()
		select {
		case errChan <- err:
		default:
		}
	}

	go func() {
		defer cancel()
		defer close(eventChan)
		defer close(errChan)

		var wg sync.WaitGroup
		for _, watch := range watchers {
			wg.Go(func() {
				if err := watch(ctx); err != nil {
					fail(err)
				}
			})
		}
		wg.Wait()
	}()

	return eventChan, errChan
}
