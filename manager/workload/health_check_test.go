package workload

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/agent/store/mocks"
)

func TestHealthCheck(t *testing.T) {
	manager := newMockWorkloadManager(t)
	ctx := context.Background()
	manager.checkAllWorkloads(ctx)
	store := manager.store.(*mocks.MockStore)
	time.Sleep(2 * time.Second)

	assertInitStatus(t, store)
}
