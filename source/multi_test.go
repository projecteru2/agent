package source

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/types"
)

var errBroken = errors.New("runtime is down")

func TestNewMultiSkipsTheFanOutForOneRuntime(t *testing.T) {
	only := &fakeSource{}
	assert.Same(t, only, NewMulti(only))

	assert.IsType(t, &Multi{}, NewMulti(only, &fakeSource{}))
}

func TestMultiListsEveryRuntime(t *testing.T) {
	multi := &Multi{sources: []Source{
		&fakeSource{workloads: []*Workload{{ID: "container"}}},
		&fakeSource{workloads: []*Workload{{ID: "process"}}},
	}}

	workloads, err := multi.List(t.Context())
	require.NoError(t, err)
	require.Len(t, workloads, 2)
	assert.Equal(t, "container", workloads[0].ID)
	assert.Equal(t, "process", workloads[1].ID)
}

func TestMultiListFailsWhenARuntimeFails(t *testing.T) {
	multi := &Multi{sources: []Source{
		&fakeSource{workloads: []*Workload{{ID: "container"}}},
		&fakeSource{err: errBroken},
	}}

	_, err := multi.List(t.Context())
	assert.ErrorIs(t, err, errBroken)
}

func TestMultiGetAsksEveryRuntime(t *testing.T) {
	multi := &Multi{sources: []Source{
		&fakeSource{err: errBroken},
		&fakeSource{workloads: []*Workload{{ID: "process"}}},
	}}

	w, err := multi.Get(t.Context(), "process")
	require.NoError(t, err)
	assert.Equal(t, "process", w.ID)

	_, err = multi.Get(t.Context(), "nowhere")
	assert.Error(t, err)
}

func TestMultiGetWithoutARuntimeThatKnowsIt(t *testing.T) {
	_, err := (&Multi{}).Get(t.Context(), "nowhere")
	assert.ErrorIs(t, err, ErrUnknownWorkload)

	multi := &Multi{sources: []Source{&fakeSource{err: errBroken}, &fakeSource{}}}
	_, err = multi.Get(t.Context(), "nowhere")
	assert.ErrorIs(t, err, ErrUnknownWorkload)
	assert.ErrorIs(t, err, errBroken)
}

func TestMultiIsAliveOnlyWhenEveryRuntimeIs(t *testing.T) {
	up, down := &fakeSource{alive: true}, &fakeSource{}

	assert.True(t, (&Multi{sources: []Source{up, up}}).Alive(t.Context()))
	assert.False(t, (&Multi{sources: []Source{up, down}}).Alive(t.Context()))
}

func TestMultiMergesTheEventsOfEveryRuntime(t *testing.T) {
	multi := &Multi{sources: []Source{
		&fakeSource{event: "container"},
		&fakeSource{event: "process"},
	}}

	events, _ := multi.Events(t.Context())
	seen := map[string]bool{}
	for range 2 {
		select {
		case event := <-events:
			seen[event.ID] = true
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the merged event stream")
		}
	}
	assert.Equal(t, map[string]bool{"container": true, "process": true}, seen)
}

func TestMultiStopsEveryRuntimeWhenOneFails(t *testing.T) {
	multi := &Multi{sources: []Source{&fakeSource{event: "container"}, &fakeSource{err: errBroken}}}

	events, errs := multi.Events(t.Context())
	var got error
	for events != nil || errs != nil {
		select {
		case _, ok := <-events:
			if !ok {
				events = nil
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			got = err
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the merged event stream to close")
		}
	}
	assert.ErrorIs(t, got, errBroken)
}

func TestMultiDeliversTheErrorWhenBothChannelsClose(t *testing.T) {
	for range 100 {
		multi := &Multi{sources: []Source{&closedFailureSource{}, &fakeSource{}}}
		_, errs := multi.Events(t.Context())
		select {
		case err := <-errs:
			assert.ErrorIs(t, err, errBroken)
		case <-time.After(5 * time.Second):
			t.Fatal("the merged stream swallowed the failure")
		}
	}
}

type fakeSource struct {
	workloads []*Workload
	event     string
	err       error
	alive     bool
}

func (f *fakeSource) List(context.Context) ([]*Workload, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.workloads, nil
}

func (f *fakeSource) Get(_ context.Context, ID string) (*Workload, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, w := range f.workloads {
		if w.ID == ID {
			return w, nil
		}
	}
	return nil, ErrUnknownWorkload
}

func (f *fakeSource) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	events := make(chan *types.WorkloadEventMessage, 1)
	errs := make(chan error, 1)
	if f.err != nil {
		errs <- f.err
	}
	if f.event != "" {
		events <- &types.WorkloadEventMessage{ID: f.event}
	}
	go func() {
		<-ctx.Done()
		close(events)
	}()
	return events, errs
}

func (f *fakeSource) Alive(context.Context) bool {
	return f.alive
}

type closedFailureSource struct {
	fakeSource
}

func (f *closedFailureSource) Events(context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	events := make(chan *types.WorkloadEventMessage)
	errs := make(chan error, 1)
	errs <- errBroken
	close(events)
	close(errs)
	return events, errs
}
