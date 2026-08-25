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
	status, err := store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, false)

	src.SetDaemonRunning(true)
	manager.nodeStatusReport(ctx)
	status, err = store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, true)
}

func TestHeartbeat(t *testing.T) {
	ctx := t.Context()
	manager := newMockNodeManager(t)
	store := manager.store.(*storemocks.MockStore)

	status, err := store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, false)

	go manager.heartbeat(ctx)

	time.Sleep(time.Duration(manager.config.HeartbeatInterval+2) * time.Second)
	status, err = store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, true)
}
