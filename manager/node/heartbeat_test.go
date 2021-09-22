package node

import (
	"context"
	"testing"

	runtimemocks "github.com/projecteru2/agent/runtime/mocks"
	storemocks "github.com/projecteru2/agent/store/mocks"

	"github.com/stretchr/testify/assert"
)

func TestNodeStatusReport(t *testing.T) {
	ctx := context.Background()
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
