package source

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/projecteru2/agent/types"
)

// Multi presents every runtime a node hosts as one source.
type Multi struct {
	sources []Source
}

// NewMulti fans out over the runtimes a node hosts; a node with one runtime needs no fan-out.
func NewMulti(sources ...Source) Source {
	if len(sources) == 1 {
		return sources[0]
	}
	return &Multi{sources: sources}
}

func (m *Multi) List(ctx context.Context) ([]*Workload, error) {
	var workloads []*Workload
	for _, src := range m.sources {
		listed, err := src.List(ctx)
		if err != nil {
			return nil, err
		}
		workloads = append(workloads, listed...)
	}
	return workloads, nil
}

// Get asks every runtime in turn, since a workload id says nothing about which one runs it.
func (m *Multi) Get(ctx context.Context, ID string) (*Workload, error) {
	refusals := []error{ErrUnknownWorkload}
	for _, src := range m.sources {
		w, err := src.Get(ctx, ID)
		if err == nil {
			return w, nil
		}
		refusals = append(refusals, err)
	}
	return nil, errors.Join(refusals...)
}

func (m *Multi) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error, len(m.sources))

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		defer close(eventChan)
		defer close(errChan)

		// one runtime failing stops them all, so the manager resubscribes to every runtime at once
		var wg sync.WaitGroup
		for _, src := range m.sources {
			events, errs := src.Events(ctx)
			wg.Go(func() {
				for {
					select {
					case event, ok := <-events:
						if !ok {
							return
						}
						select {
						case eventChan <- event:
						case <-ctx.Done():
							return
						}
					case err, ok := <-errs:
						if ok {
							select {
							case errChan <- err:
							default:
							}
							cancel()
						}
						return
					case <-ctx.Done():
						return
					}
				}
			})
		}
		wg.Wait()
	}()

	return eventChan, errChan
}

// Alive reports whether every runtime the node hosts is up.
func (m *Multi) Alive(ctx context.Context) bool {
	return !slices.ContainsFunc(m.sources, func(src Source) bool { return !src.Alive(ctx) })
}
