package workload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/source"
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
	require.NoError(t, store.SetWorkloadStatus(t.Context(), status, 0))

	manager.checkAllWorkloads(t.Context())

	got := store.GetMockWorkloadStatus(status.ID)
	require.NotNil(t, got)
	assert.False(t, got.Running)
	assert.Empty(t, manager.collecting)
	assert.Empty(t, manager.logTargets)
}
