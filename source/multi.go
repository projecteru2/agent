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
	owners  sync.Map
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

// Get asks the runtime that answered for the id last, then every other one, since an id says nothing about which runtime runs it.
func (m *Multi) Get(ctx context.Context, ID string) (*Workload, error) {
	if owner, ok := m.owners.Load(ID); ok {
		if w, err := owner.(Source).Get(ctx, ID); err == nil {
			return w, nil
		}
		m.owners.Delete(ID)
	}
	refusals := []error{ErrUnknownWorkload}
	for _, src := range m.sources {
		w, err := src.Get(ctx, ID)
		if err == nil {
			m.owners.Store(ID, src)
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
				for events != nil || errs != nil {
					select {
					case event, ok := <-events:
						if !ok {
							events = nil
							continue
						}
						select {
						case eventChan <- event:
						case <-ctx.Done():
							return
						}
					case err, ok := <-errs:
						if !ok {
							errs = nil
							continue
						}
						select {
						case errChan <- err:
						default:
						}
						cancel()
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

func (m *Multi) Refresh(ID string) (*Workload, error) {
	refusals := []error{ErrUnknownWorkload}
	for _, src := range m.sources {
		refresher, ok := src.(Refresher)
		if !ok {
			continue
		}
		w, err := refresher.Refresh(ID)
		if err == nil {
			return w, nil
		}
		refusals = append(refusals, err)
	}
	return nil, errors.Join(refusals...)
}

// Alive reports whether every runtime the node hosts is up.
func (m *Multi) Alive(ctx context.Context) bool {
	return !slices.ContainsFunc(m.sources, func(src Source) bool { return !src.Alive(ctx) })
}
