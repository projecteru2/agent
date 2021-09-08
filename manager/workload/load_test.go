package workload

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"

	"github.com/stretchr/testify/assert"
)

func assertInitStatus(t *testing.T, store *mocks.MockStore) {
	assert.Equal(t, store.GetMockWorkloadStatus("Asuka"), &types.WorkloadStatus{
		ID:      "Asuka",
		Running: false,
		Healthy: false,
	})

	assert.Equal(t, store.GetMockWorkloadStatus("Rei"), &types.WorkloadStatus{
		ID:      "Rei",
		Running: true,
		Healthy: false,
	})

	assert.Equal(t, store.GetMockWorkloadStatus("Shinji"), &types.WorkloadStatus{
		ID:      "Shinji",
		Running: true,
		Healthy: true,
	})
}

func TestLoad(t *testing.T) {
	manager := newMockWorkloadManager(t)
	store := manager.store.(*mocks.MockStore)
	ctx := context.Background()
	err := manager.load(ctx)
	// wait for attaching
	time.Sleep(2 * time.Second)
	assert.Nil(t, err)
	assertInitStatus(t, store)
}
