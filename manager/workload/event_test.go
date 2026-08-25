package workload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sourcemocks "github.com/projecteru2/agent/source/mocks"
	storemocks "github.com/projecteru2/agent/store/mocks"
)

func TestEvent(t *testing.T) {
	ctx := t.Context()

	manager := newMockWorkloadManager(t)
	src := manager.source.(*sourcemocks.Nerv)
	store := manager.store.(*storemocks.MockStore)
	assert.Nil(t, manager.initWorkloadStatus(ctx))
	assertInitStatus(t, store)

	go manager.monitor(ctx)

	go src.StartEvents()
	time.Sleep(5 * time.Second)

	assert.Equal(t, store.GetMockWorkloadStatus("Asuka"), wantStatus("Asuka", "eva2", false, false))
	assert.Equal(t, store.GetMockWorkloadStatus("Rei"), wantStatus("Rei", "eva0", false, false))
	assert.Equal(t, store.GetMockWorkloadStatus("Shinji"), wantStatus("Shinji", "eva1", true, true))
}
