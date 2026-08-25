package workload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/store/mocks"
)

func TestHealthCheck(t *testing.T) {
	manager := newMockWorkloadManager(t)
	ctx := t.Context()
	manager.checkAllWorkloads(ctx)
	store := manager.store.(*mocks.MockStore)
	time.Sleep(2 * time.Second)

	assertInitStatus(t, store)
}

func TestCheckOneWorkloadStartsTheSamplerAStartEventMissed(t *testing.T) {
	manager := newMockWorkloadManager(t)
	w := &source.Workload{ID: "Rei", CgroupPath: t.TempDir()}

	manager.checkOneWorkload(t.Context(), w)
	assert.Empty(t, manager.collecting)

	w.Running = true
	manager.checkOneWorkload(t.Context(), w)
	assert.NotNil(t, manager.collecting[w.ID])
}
