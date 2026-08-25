package workload

import (
	"testing"
	"time"

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
