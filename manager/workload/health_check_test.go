package workload

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
)

func TestHealthCheck(t *testing.T) {
	manager := newMockWorkloadManager(t)
	ctx := t.Context()
	manager.checkAllWorkloads(ctx)
	store := manager.store.(*mocks.MockStore)

	assertInitStatus(t, store)
}

func TestCheckOneWorkloadRepairsMissedEvents(t *testing.T) {
	manager := newMockWorkloadManager(t)
	w := &source.Workload{ID: "Rei", CgroupPath: t.TempDir()}

	manager.checkOneWorkload(t.Context(), w)
	assert.Empty(t, manager.collecting)

	w.Running = true
	manager.checkOneWorkload(t.Context(), w)
	assert.NotNil(t, manager.collecting[w.ID])
	assert.NotEmpty(t, manager.logTargets)

	w.Running = false
	manager.checkOneWorkload(t.Context(), w)
	assert.Empty(t, manager.collecting)
	assert.Empty(t, manager.logTargets)
}

func TestHealthCheckReportsCoreRunningWorkloadMissingFromRuntime(t *testing.T) {
	manager := newMockWorkloadManager(t)
	manager.source = &forgetfulSource{}
	store := manager.store.(*mocks.MockStore)
	w := &source.Workload{
		ID:         "Kaworu",
		Meta:       source.Meta{Appname: "nerv", Entrypoint: "eva3"},
		CgroupPath: t.TempDir(),
		Running:    true,
	}
	manager.start(t.Context(), w)
	status := &types.WorkloadStatus{
		ID:         w.ID,
		Running:    true,
		Healthy:    true,
		Appname:    "nerv",
		Nodename:   "fake",
		Entrypoint: "eva3",
	}
	require.NoError(t, store.SetWorkloadStatus(t.Context(), status))

	manager.checkAllWorkloads(t.Context())

	got := store.GetMockWorkloadStatus(status.ID)
	require.NotNil(t, got)
	assert.False(t, got.Running)
	assert.Empty(t, manager.collecting)
	assert.Empty(t, manager.logTargets)
}

func TestHealthCheckKeepsAWorkloadTheListingMissed(t *testing.T) {
	manager := newMockWorkloadManager(t)
	w := &source.Workload{ID: "Misato", CgroupPath: t.TempDir(), Running: true}
	manager.source = &blindListSource{sweepRaceSource{workloads: []*source.Workload{w}}}
	store := manager.store.(*mocks.MockStore)
	status := &types.WorkloadStatus{ID: w.ID, Running: true, Healthy: true, Nodename: "fake"}
	require.NoError(t, store.SetWorkloadStatus(t.Context(), status))

	manager.checkAllWorkloads(t.Context())

	got := store.GetMockWorkloadStatus(w.ID)
	require.NotNil(t, got)
	assert.True(t, got.Running)
	assert.NotNil(t, manager.collecting[w.ID])
	manager.stop(w.ID)
}

func TestHealthCheckStopsAnOrphanedLocalTask(t *testing.T) {
	manager := newMockWorkloadManager(t)
	manager.source = &forgetfulSource{}
	w := &source.Workload{
		ID:         "Kaworu",
		CgroupPath: t.TempDir(),
		Running:    true,
		Log:        source.Log{JournalUnit: "eru-Kaworu.service"},
	}
	manager.start(t.Context(), w)

	manager.checkAllWorkloads(t.Context())

	assert.Empty(t, manager.collecting)
	assert.Empty(t, manager.logTargets)
}

func TestHealthCheckSparesAnOrphanTheRuntimeStillKnows(t *testing.T) {
	manager := newMockWorkloadManager(t)
	w := &source.Workload{ID: "Misato", CgroupPath: t.TempDir(), Running: true}
	manager.source = &blindListSource{sweepRaceSource{workloads: []*source.Workload{w}}}
	manager.start(t.Context(), w)

	manager.checkAllWorkloads(t.Context())

	assert.NotNil(t, manager.collecting[w.ID])
	manager.stop(w.ID)
}

func TestHealthCheckSamplesAWorkloadStartingMidSweep(t *testing.T) {
	manager := newMockWorkloadManager(t)
	src := &sweepRaceSource{}
	manager.source = src
	w := &source.Workload{ID: "Misato", CgroupPath: t.TempDir(), Running: true}
	status := &types.WorkloadStatus{ID: w.ID, Running: true, Healthy: true, Nodename: "fake"}
	require.NoError(t, manager.store.SetWorkloadStatus(t.Context(), status))
	manager.store = &sweepRaceStore{Store: manager.store, src: src, starting: w}

	manager.checkAllWorkloads(t.Context())

	assert.NotNil(t, manager.collecting[w.ID])
	assert.NotEmpty(t, manager.logTargets)
}

type sweepRaceSource struct {
	source.Source
	workloads []*source.Workload
}

func (s *sweepRaceSource) List(context.Context) ([]*source.Workload, error) {
	return s.workloads, nil
}

func (s *sweepRaceSource) Get(_ context.Context, ID string) (*source.Workload, error) {
	if i := slices.IndexFunc(s.workloads, func(w *source.Workload) bool { return w.ID == ID }); i >= 0 {
		return s.workloads[i], nil
	}
	return nil, errors.New("unknown workload")
}

type blindListSource struct {
	sweepRaceSource
}

func (s *blindListSource) List(context.Context) ([]*source.Workload, error) {
	return nil, nil
}

type sweepRaceStore struct {
	store.Store
	src      *sweepRaceSource
	starting *source.Workload
}

func (s *sweepRaceStore) ListRunningWorkloadIDs(ctx context.Context) ([]string, error) {
	IDs, err := s.Store.ListRunningWorkloadIDs(ctx)
	s.src.workloads = append(s.src.workloads, s.starting)
	return IDs, err
}
