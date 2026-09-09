package workload

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/manager"
	"github.com/projecteru2/agent/source/mocks"
	"github.com/projecteru2/agent/types"
)

const (
	runTimeout       = 30 * time.Second
	connectTimeout   = 5 * time.Second
	journalDrainWait = 2 * connectTimeout
)

func TestRun(t *testing.T) {
	t.Setenv("PATH", "")
	synctest.Test(t, func(t *testing.T) {
		manager := newMockWorkloadManager(t)
		src := manager.source.(*mocks.Nerv)
		ctx, cancel := context.WithTimeout(t.Context(), runTimeout)
		defer cancel()

		go func() {
			src.StartEvents()
			src.StartCustomEvent(&types.WorkloadEventMessage{
				ID:     "Kaworu",
				Action: "start",
			})
		}()

		assert.Nil(t, manager.Run(ctx))
		synctest.Sleep(journalDrainWait)
	})
}

func newMockWorkloadManager(t *testing.T) *Manager {
	config := &types.Config{
		HostName:          "fake",
		HeartbeatInterval: 10,
		CheckOnlyMine:     false,
		Store:             common.MocksStore,
		Runtimes:          types.RuntimesConfig{Mocks: &types.MocksConfig{}},
		Metrics:           types.MetricsConfig{Step: 10},
		Log: types.LogConfig{
			Stdout: true,
		},
		HealthCheck: types.HealthCheckConfig{
			Interval: 10,
			Timeout:  5,
			CacheTTL: 300,
		},
		GlobalConnectionTimeout: connectTimeout,
	}

	clients, err := manager.NewClients(t.Context(), config)
	assert.Nil(t, err)
	return NewManager(t.Context(), config, clients)
}
