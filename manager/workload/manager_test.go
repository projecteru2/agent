package workload

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/runtime/mocks"
	"github.com/projecteru2/agent/types"
)

func TestRun(t *testing.T) {
	manager := newMockWorkloadManager(t)
	runtime := manager.runtimeClient.(*mocks.Nerv)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
	defer cancel()
	go func() {
		runtime.StartEvents()
		runtime.StartCustomEvent(&types.WorkloadEventMessage{
			ID:     "Kaworu",
			Action: "start",
		})
	}()
	assert.Nil(t, manager.Run(ctx))
}

func newMockWorkloadManager(t *testing.T) *Manager {
	config := &types.Config{
		HostName:          "fake",
		HeartbeatInterval: 10,
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
