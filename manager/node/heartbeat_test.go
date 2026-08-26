package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sourcemocks "github.com/projecteru2/agent/source/mocks"
	storemocks "github.com/projecteru2/agent/store/mocks"
)

func TestNodeStatusReport(t *testing.T) {
	ctx := t.Context()
	manager := newMockNodeManager(t)
	src := manager.source.(*sourcemocks.Nerv)
	store := manager.store.(*storemocks.MockStore)

	src.SetDaemonRunning(false)
	manager.nodeStatusReport(ctx)
	status := store.GetMockNodeStatus("fake")
	assert.Equal(t, status.Alive, false)

	src.SetDaemonRunning(true)
	manager.nodeStatusReport(ctx)
	status = store.GetMockNodeStatus("fake")
	assert.Equal(t, status.Alive, true)
}

func TestHeartbeat(t *testing.T) {
	ctx := t.Context()
	manager := newMockNodeManager(t)
	store := manager.store.(*storemocks.MockStore)

	status := store.GetMockNodeStatus("fake")
	assert.Equal(t, status.Alive, false)

	go manager.heartbeat(ctx)

	time.Sleep(time.Duration(manager.config.HeartbeatInterval+2) * time.Second)
	status = store.GetMockNodeStatus("fake")
	assert.Equal(t, status.Alive, true)
}
