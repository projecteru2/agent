package workload

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/runtime/mocks"
	"github.com/projecteru2/agent/types"

	"github.com/stretchr/testify/assert"
)

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
			Interval:      10,
			Timeout:       5,
			CacheTTL:      300,
			EnableSelfmon: true,
		},
		GlobalConnectionTimeout: 5 * time.Second,
	}

	m, err := NewManager(context.Background(), config)
	assert.Nil(t, err)
	return m
}

func TestRun(t *testing.T) {
	manager := newMockWorkloadManager(t)
	runtime := manager.runtimeClient.(*mocks.Nerv)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
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
