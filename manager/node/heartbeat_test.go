package node

import (
	"context"
	"testing"
	"time"

	runtimemocks "github.com/projecteru2/agent/runtime/mocks"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/utils"

	"github.com/stretchr/testify/assert"
)

func TestNodeStatusReport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := newMockNodeManager(t)
	runtime := manager.runtimeClient.(*runtimemocks.Nerv)
	store := manager.store.(*storemocks.MockStore)

	runtime.SetDaemonRunning(false)
	manager.nodeStatusReport(ctx)
	status, err := store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, false)

	runtime.SetDaemonRunning(true)
	manager.nodeStatusReport(ctx)
	status, err = store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, true)
}

func TestHeartbeat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	utils.NewPool(1000)
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
