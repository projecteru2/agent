package node

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
)

func TestRun(t *testing.T) {
	manager := newMockNodeManager(t)
	store := manager.store.(*storemocks.MockStore)

	ctx, cancel := context.WithTimeout(t.Context(), time.Duration(manager.config.HeartbeatInterval*3)*time.Second)
	defer cancel()

	status, err := store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, false)

	go func() {
		time.Sleep(time.Duration(manager.config.HeartbeatInterval*2) * time.Second)
		status, err := store.GetNodeStatus(ctx, "fake")
		assert.Nil(t, err)
		assert.Equal(t, status.Alive, true)
	}()

	assert.Nil(t, manager.Run(ctx))

	info, err := store.GetNode(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, info.Available, false)
}

func newMockNodeManager(t *testing.T) *Manager {
	config := &types.Config{
		HostName:          "fake",
		HeartbeatInterval: 2,
		CheckOnlyMine:     false,
		Store:             common.MocksStore,
		Runtime:           common.MocksRuntime,
		Log: types.LogConfig{
			Stdout: true,
		},
		HealthCheck: types.HealthCheckConfig{
			Interval: 10,
			Timeout:  5,
			CacheTTL: 300,
		},
		GlobalConnectionTimeout: 5 * time.Second,
	}

	m, err := NewManager(t.Context(), config)
	assert.Nil(t, err)
	return m
}
