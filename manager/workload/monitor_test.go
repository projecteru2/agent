package workload

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

func TestHandleWorkloadDieCarriesTheForwardedMeta(t *testing.T) {
	manager := newMockWorkloadManager(t)
	manager.source = &forgetfulSource{}
	store := manager.store.(*mocks.MockStore)
	w := &source.Workload{
		ID:   "Kaworu",
		Meta: source.Meta{Appname: "nerv", Entrypoint: "eva3"},
		Log:  source.Log{JournalUnit: "eru-Kaworu.service"},
	}
	manager.startForwarding(t.Context(), w)

	manager.handleWorkloadDie(t.Context(), &types.WorkloadEventMessage{ID: w.ID, Action: "die"})

	status := store.GetMockWorkloadStatus(w.ID)
	require.NotNil(t, status)
	assert.False(t, status.Running)
	assert.Equal(t, "nerv", status.Appname)
	assert.Equal(t, "eva3", status.Entrypoint)
}

func TestHandleWorkloadStartRetriesAGetTheRuntimeCannotAnswerYet(t *testing.T) {
	manager := newMockWorkloadManager(t)
	w := &source.Workload{ID: "Rei", CgroupPath: t.TempDir(), Running: true}
	manager.source = &lateSource{w: w, failures: 1}

	manager.handleWorkloadStart(t.Context(), &types.WorkloadEventMessage{ID: w.ID, Action: "start"})

	assert.Eventually(t, func() bool {
		manager.collectMutex.Lock()
		defer manager.collectMutex.Unlock()
		_, ok := manager.collecting[w.ID]
		return ok
	}, 5*time.Second, 50*time.Millisecond)
	manager.stop(w.ID)
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

type lateSource struct {
	source.Source
	mu       sync.Mutex
	failures int
	w        *source.Workload
}

func (s *lateSource) Get(context.Context, string) (*source.Workload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failures > 0 {
		s.failures--
		return nil, errors.New("unit not loaded yet")
	}
	return s.w, nil
}
