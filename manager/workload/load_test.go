package workload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
)

func TestLoad(t *testing.T) {
	manager := newMockWorkloadManager(t)
	store := manager.store.(*mocks.MockStore)
	ctx := t.Context()
	err := manager.initWorkloadStatus(ctx)
	time.Sleep(2 * time.Second)
	assert.Nil(t, err)
	assertInitStatus(t, store)
}

func assertInitStatus(t *testing.T, store *mocks.MockStore) {
	assert.Equal(t, store.GetMockWorkloadStatus("Asuka"), wantStatus("Asuka", "eva2", false, false))
	assert.Equal(t, store.GetMockWorkloadStatus("Rei"), wantStatus("Rei", "eva0", true, false))
	assert.Equal(t, store.GetMockWorkloadStatus("Shinji"), wantStatus("Shinji", "eva1", true, true))
}

func wantStatus(ID, entrypoint string, running, healthy bool) *types.WorkloadStatus {
	return &types.WorkloadStatus{
		ID:         ID,
		Running:    running,
		Healthy:    healthy,
		Extension:  []byte("null"),
		Appname:    "nerv",
		Nodename:   "fake",
		Entrypoint: entrypoint,
	}
}
