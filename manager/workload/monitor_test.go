package workload

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
)

func TestHandleWorkloadDieReportsAWorkloadTheRuntimeForgot(t *testing.T) {
	manager := newMockWorkloadManager(t)
	manager.source = &forgetfulSource{}
	store := manager.store.(*mocks.MockStore)

	manager.handleWorkloadDie(t.Context(), &types.WorkloadEventMessage{ID: "Kaworu", Action: "die"})

	status := store.GetMockWorkloadStatus("Kaworu")
	require.NotNil(t, status)
	assert.False(t, status.Running)
	assert.False(t, status.Healthy)
	assert.Equal(t, "fake", status.Nodename)
}

func TestHandleWorkloadDieReportsWhatTheRuntimeStillKnows(t *testing.T) {
	manager := newMockWorkloadManager(t)
	store := manager.store.(*mocks.MockStore)

	manager.handleWorkloadDie(t.Context(), &types.WorkloadEventMessage{ID: "Asuka", Action: "die"})

	status := store.GetMockWorkloadStatus("Asuka")
	require.NotNil(t, status)
	assert.False(t, status.Running)
	assert.Equal(t, "nerv", status.Appname)
}

type forgetfulSource struct {
	source.Source
}

func (f *forgetfulSource) List(context.Context) ([]*source.Workload, error) {
	return nil, nil
}

func (f *forgetfulSource) Get(context.Context, string) (*source.Workload, error) {
	return nil, errors.New("no runtime on this node knows this workload")
}
