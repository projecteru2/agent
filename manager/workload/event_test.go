package workload

import (
	"context"
	"testing"
	"time"

	runtimemocks "github.com/projecteru2/agent/runtime/mocks"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"

	"github.com/stretchr/testify/assert"
)

func TestEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := newMockWorkloadManager(t)
	runtime := manager.runtimeClient.(*runtimemocks.Nerv)
	store := manager.store.(*storemocks.MockStore)
	// init workload status
	assert.Nil(t, manager.initWorkloadStatus(ctx))
	assertInitStatus(t, store)

	go manager.monitor(ctx)

	// starts the events: Shinji 400%, Asuka starts, Asuka dies, Rei dies
	go runtime.StartEvents()
	time.Sleep(5 * time.Second)

	assert.Equal(t, store.GetMockWorkloadStatus("Asuka"), &types.WorkloadStatus{
		ID:      "Asuka",
		Running: false,
		Healthy: false,
	})

	assert.Equal(t, store.GetMockWorkloadStatus("Rei"), &types.WorkloadStatus{
		ID:      "Rei",
		Running: false,
		Healthy: false,
	})

	assert.Equal(t, store.GetMockWorkloadStatus("Shinji"), &types.WorkloadStatus{
		ID:      "Shinji",
		Running: true,
		Healthy: true,
	})
}
